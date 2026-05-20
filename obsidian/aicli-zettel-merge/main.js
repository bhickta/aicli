const { Modal, Notice, Plugin, PluginSettingTab, Setting, requestUrl } = require("obsidian");

const DEFAULT_SETTINGS = {
  baseUrl: "http://127.0.0.1:8765",
  vaultPath: "",
  rootFolder: "zettelkasten",
  dataFolder: ".aicli-zettel-merge",
  providerId: "lms",
  embeddingProviderId: "lms",
  judgeModel: "deepseek-reasoner",
  mergeModel: "deepseek-reasoner",
  embeddingModel: "text-embedding-nomic-embed-text-v1.5",
  candidateLimit: 12,
  reviewThreshold: 0.85,
  validationThreshold: 0.98,
  candidateJudgeChars: 2500,
  maxMergeInputChars: 120000
};

module.exports = class AICLIZettelMergePlugin extends Plugin {
  async onload() {
    this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
    this.addRibbonIcon("git-merge", "AICLI Zettel Merge", () => this.openMergeModal());
    this.addCommand({
      id: "suggest-zettel-merges",
      name: "Suggest Zettel Merges",
      callback: () => this.openMergeModal(true)
    });
    this.addCommand({
      id: "rollback-latest-zettel-merge",
      name: "Rollback Latest Zettel Merge",
      callback: () => this.rollbackLatest()
    });
    this.addSettingTab(new AICLIZettelMergeSettingTab(this.app, this));
  }

  async saveSettings() {
    await this.saveData(this.settings);
  }

  openMergeModal(autoSuggest = false) {
    const activeFile = this.app.workspace.getActiveFile();
    if (!activeFile || activeFile.extension !== "md") {
      new Notice("Open a Zettelkasten markdown note first.");
      return;
    }
    new ZettelMergeModal(this.app, this, activeFile, autoSuggest).open();
  }

  async rollbackLatest() {
    try {
      const client = new AICLIClient(this);
      const result = await client.runWorkflow("/api/workflows/zettel/rollback", this.basePayload());
      new Notice(`Rolled back zettel merge job ${result.job_id}.`, 10000);
    } catch (error) {
      new Notice(`Rollback failed: ${error.message}`, 10000);
    }
  }

  basePayload() {
    return {
      vault_path: this.vaultPath(),
      root_folder: this.settings.rootFolder,
      data_folder: this.settings.dataFolder,
      provider_id: this.settings.providerId,
      embedding_provider_id: this.settings.embeddingProviderId,
      judge_model: this.settings.judgeModel,
      merge_model: this.settings.mergeModel,
      embedding_model: this.settings.embeddingModel,
      candidate_limit: Number(this.settings.candidateLimit) || DEFAULT_SETTINGS.candidateLimit,
      review_threshold: Number(this.settings.reviewThreshold) || DEFAULT_SETTINGS.reviewThreshold,
      validation_threshold: Number(this.settings.validationThreshold) || DEFAULT_SETTINGS.validationThreshold,
      candidate_judge_chars: Number(this.settings.candidateJudgeChars) || DEFAULT_SETTINGS.candidateJudgeChars,
      max_merge_input_chars: Number(this.settings.maxMergeInputChars) || DEFAULT_SETTINGS.maxMergeInputChars
    };
  }

  vaultPath() {
    if (this.settings.vaultPath.trim()) return this.settings.vaultPath.trim();
    const adapter = this.app.vault.adapter;
    if (adapter && typeof adapter.getBasePath === "function") return adapter.getBasePath();
    return "";
  }
};

class AICLIClient {
  constructor(plugin) {
    this.plugin = plugin;
  }

  async request(path, body) {
    const base = this.plugin.settings.baseUrl.replace(/\/+$/, "");
    const response = await requestUrl({
      url: base + path,
      method: body === undefined ? "GET" : "POST",
      contentType: "application/json",
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
      throw: false
    });
    if (response.status < 200 || response.status >= 300) {
      const text = response.text || JSON.stringify(response.json || {});
      throw new Error(`[${response.status}] ${text}`);
    }
    return response.json;
  }

  async runWorkflow(path, body, onProgress) {
    const started = await this.request(path, body);
    const job = started.job;
    if (!job || !job.id) throw new Error("AICLI did not return a job id.");
    onProgress?.(job);
    for (;;) {
      await sleep(1000);
      const current = await this.request(`/api/jobs/${encodeURIComponent(job.id)}`);
      onProgress?.(current);
      if (current.status === "completed") {
        return current.output ? JSON.parse(current.output) : {};
      }
      if (current.status === "failed") {
        throw new Error(current.error || "workflow failed");
      }
    }
  }
}

class ZettelMergeModal extends Modal {
  constructor(app, plugin, activeFile, autoSuggest) {
    super(app);
    this.plugin = plugin;
    this.client = new AICLIClient(plugin);
    this.activeFile = activeFile;
    this.autoSuggest = autoSuggest;
    this.suggestions = [];
    this.selected = new Set();
    this.proposal = null;
    this.status = "Ready";
    this.busy = false;
  }

  onOpen() {
    this.contentEl.addClass("aicli-zettel-modal");
    this.render();
    if (this.autoSuggest) void this.suggest();
  }

  render() {
    this.contentEl.empty();
    const selectedCount = this.selected.size;
    this.contentEl.createEl("h2", { text: "AICLI Zettel Merge" });
    this.contentEl.createDiv({
      cls: "aicli-zettel-subtitle",
      text: this.activeFile.path
    });

    const toolbar = this.contentEl.createDiv({ cls: "aicli-zettel-toolbar" });
    this.button(toolbar, "Suggest", () => this.suggest(), !this.busy);
    this.button(toolbar, "Build Index", () => this.index(), !this.busy);
    this.button(toolbar, `Preview Merge${selectedCount ? ` (${selectedCount})` : ""}`, () => this.propose(), !this.busy && selectedCount > 0);
    this.button(toolbar, "Apply", () => this.apply(), !this.busy && Boolean(this.proposal), "mod-cta");
    this.button(toolbar, "Rollback", () => this.rollback(), !this.busy);

    const status = this.contentEl.createDiv({ cls: "aicli-zettel-status", text: this.status });
    status.setAttr("role", "status");
    status.setAttr("aria-live", "polite");

    const body = this.contentEl.createDiv({ cls: "aicli-zettel-body" });
    const left = body.createDiv({ cls: "aicli-zettel-candidates" });
    const right = body.createDiv({ cls: "aicli-zettel-preview" });
    this.renderCandidates(left);
    this.renderPreview(right);
  }

  renderCandidates(container) {
    container.createEl("h3", { text: "Candidates" });
    if (!this.suggestions.length) {
      container.createDiv({
        cls: "aicli-zettel-empty",
        text: "Run Suggest to find mergeable notes from the AICLI engine."
      });
      return;
    }
    for (const candidate of this.suggestions) {
      const card = container.createDiv({ cls: "aicli-zettel-card" });
      const header = card.createDiv({ cls: "aicli-zettel-card-header" });
      const checkbox = header.createEl("input", { type: "checkbox" });
      checkbox.checked = this.selected.has(candidate.path);
      checkbox.onchange = () => {
        if (checkbox.checked) this.selected.add(candidate.path);
        else this.selected.delete(candidate.path);
        this.proposal = null;
        this.render();
      };
      header.createDiv({ cls: "aicli-zettel-path", text: candidate.path });
      const badges = card.createDiv({ cls: "aicli-zettel-badges" });
      this.badge(badges, `sim ${formatScore(candidate.similarity)}`);
      this.badge(badges, `conf ${formatScore(candidate.confidence)}`);
      this.badge(badges, candidate.relationship || "relationship");
      this.badge(badges, candidate.risk || "risk");
      card.createDiv({ cls: "aicli-zettel-reason", text: candidate.reason || "No reason returned." });
      card.createDiv({ cls: "aicli-zettel-ranges", text: `Lines: ${formatRanges(candidate.source_line_ranges || [])}` });
      const excerpt = card.createEl("pre", { cls: "aicli-zettel-excerpt" });
      excerpt.textContent = candidate.extracted_markdown || "";
    }
  }

  renderPreview(container) {
    container.createEl("h3", { text: "Merge Preview" });
    if (!this.proposal) {
      container.createDiv({
        cls: "aicli-zettel-empty",
        text: "Select candidates and run Preview Merge. Nothing is written until Apply succeeds."
      });
      return;
    }
    const quality = container.createDiv({ cls: "aicli-zettel-quality" });
    this.badge(quality, `coverage ${formatScore(this.proposal.coverage?.score)}`);
    this.badge(quality, `judge ${formatScore(this.proposal.judge?.score)}`);
    this.badge(quality, this.proposal.judge?.verdict || "verdict");
    const preview = container.createEl("textarea", { cls: "aicli-zettel-final" });
    preview.readOnly = true;
    preview.value = this.proposal.final_markdown || "";
  }

  button(container, text, onClick, enabled = true, cls = "") {
    const button = container.createEl("button", { text, cls });
    button.disabled = !enabled;
    button.onclick = async () => {
      button.disabled = true;
      try {
        await onClick();
      } finally {
        button.disabled = false;
      }
    };
    return button;
  }

  badge(container, text) {
    container.createSpan({ cls: "aicli-zettel-badge", text });
  }

  async index() {
    await this.run("Building embedding index", "/api/workflows/zettel/index", this.plugin.basePayload(), () => {
      this.status = "Embedding index is ready.";
    });
  }

  async suggest() {
    const payload = Object.assign({}, this.plugin.basePayload(), { active_path: this.activeFile.path });
    await this.run("Finding candidates", "/api/workflows/zettel/suggest", payload, (result) => {
      this.suggestions = result.candidates || [];
      this.selected.clear();
      this.proposal = null;
      this.status = this.suggestions.length
        ? `${this.suggestions.length} mergeable candidate(s) found.`
        : "No mergeable candidates found.";
    });
  }

  async propose() {
    const selections = this.suggestions
      .filter((candidate) => this.selected.has(candidate.path))
      .map((candidate) => ({
        path: candidate.path,
        source_line_ranges: candidate.source_line_ranges || []
      }));
    const payload = Object.assign({}, this.plugin.basePayload(), {
      active_path: this.activeFile.path,
      selections
    });
    await this.run("Preparing merge preview", "/api/workflows/zettel/propose", payload, (result) => {
      this.proposal = result.proposal;
      this.status = "Merge preview is ready. Review before applying.";
    });
  }

  async apply() {
    if (!this.proposal) return;
    const payload = Object.assign({}, this.plugin.basePayload(), { proposal: this.proposal });
    await this.run("Applying approved merge", "/api/workflows/zettel/apply", payload, (result) => {
      this.status = `Applied merge ${result.job_id}. Archive: ${result.archive_path}`;
      new Notice(`Applied zettel merge ${result.job_id}.`, 10000);
    });
  }

  async rollback() {
    await this.run("Rolling back latest merge", "/api/workflows/zettel/rollback", this.plugin.basePayload(), (result) => {
      this.status = `Rolled back merge ${result.job_id}.`;
      new Notice(`Rolled back zettel merge ${result.job_id}.`, 10000);
    });
  }

  async run(label, endpoint, payload, onDone) {
    try {
      this.busy = true;
      this.status = `${label}...`;
      this.render();
      const result = await this.client.runWorkflow(endpoint, payload, (job) => {
        if (job.stage) {
          const pct = Number.isFinite(job.progress) ? ` ${Math.round(job.progress * 100)}%` : "";
          this.status = `${job.stage}${pct}`;
          this.render();
        }
      });
      onDone(result);
      this.render();
    } catch (error) {
      this.status = `${label} failed: ${error.message}`;
      this.render();
      new Notice(this.status, 10000);
    } finally {
      this.busy = false;
      this.render();
    }
  }
}

class AICLIZettelMergeSettingTab extends PluginSettingTab {
  constructor(app, plugin) {
    super(app, plugin);
    this.plugin = plugin;
  }

  display() {
    const { containerEl } = this;
    containerEl.empty();
    containerEl.createEl("h2", { text: "AICLI Zettel Merge" });
    this.text("AICLI URL", "Local aicli server URL.", "baseUrl");
    this.text("Vault path", "Leave empty to use the current desktop vault path.", "vaultPath");
    this.text("Zettelkasten folder", "Vault-relative folder scanned by the Go engine.", "rootFolder");
    this.text("Data folder", "Vault-relative archive/cache folder used by aicli.", "dataFolder");
    this.text("LLM provider ID", "AICLI provider id used for judging and merging, for example codex-cli.", "providerId");
    this.text("Judge model", "Model used to choose mergeable exact line ranges.", "judgeModel");
    this.text("Merge model", "Model used to write scoped insertion proposals.", "mergeModel");
    this.text("Embedding provider ID", "AICLI provider id used for note similarity embeddings, usually lms or ollama.", "embeddingProviderId");
    this.text("Embedding model", "Model used for note similarity search.", "embeddingModel");
    this.number("Candidate limit", "Top matching notes sent to the judge.", "candidateLimit");
    this.number("Review threshold", "Candidates below this confidence stay hidden.", "reviewThreshold");
    this.number("Validation threshold", "Apply requires coverage and judge score at or above this value.", "validationThreshold");
    this.number("Candidate judge chars", "Max chars per candidate sent for line-range judging.", "candidateJudgeChars");
    this.number("Max merge input chars", "Hard stop for merge prompt size.", "maxMergeInputChars");
  }

  text(name, desc, key) {
    new Setting(this.containerEl)
      .setName(name)
      .setDesc(desc)
      .addText((text) => text
        .setValue(String(this.plugin.settings[key] ?? ""))
        .onChange(async (value) => {
          this.plugin.settings[key] = value.trim();
          await this.plugin.saveSettings();
        }));
  }

  number(name, desc, key) {
    new Setting(this.containerEl)
      .setName(name)
      .setDesc(desc)
      .addText((text) => text
        .setValue(String(this.plugin.settings[key] ?? ""))
        .onChange(async (value) => {
          const parsed = Number(value);
          if (Number.isFinite(parsed)) {
            this.plugin.settings[key] = parsed;
            await this.plugin.saveSettings();
          }
        }));
  }
}

function formatScore(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return "n/a";
  return parsed.toFixed(2);
}

function formatRanges(ranges) {
  if (!ranges.length) return "none";
  return ranges.map((range) => {
    const start = range.start_line ?? range.startLine;
    const end = range.end_line ?? range.endLine;
    return start === end ? String(start) : `${start}-${end}`;
  }).join(", ");
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
