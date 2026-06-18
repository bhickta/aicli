const { Notice, Plugin, PluginSettingTab, Setting, requestUrl } = require("obsidian");

const DEFAULT_SETTINGS = {
  baseUrl: "http://127.0.0.1:8765",
  vaultPath: "",
  rootFolder: "zettelkasten",
  inboxFolder: "inbox-to-merge",
  dataFolder: ".aicli-zettel-merge",
  providerId: "lms",
  mergeModel: "deepseek-reasoner",
  embeddingProviderId: "lms",
  embeddingModel: "text-embedding-nomic-embed-text-v1.5",
  candidateLimit: 12,
  embeddingBatchSize: 128,
  embeddingWorkers: 4,
  inboxWorkers: 1,
  inboxRandom: false,
  shorthandPromptPath: "example_prompts.md",
  lectureProviderId: "lms",
  lectureModel: "",
  lectureStyle: "crisp comprehensive UPSC lecture",
  lectureMaxNotes: 25,
  lectureMaxInputChars: 120000,
  lectureSynthesizeAudio: true,
  lectureTTSCommand: "ots.TTS",
  lectureTTSArgs: 'SOAR --input "{script}" --output "{audio}"'
};

module.exports = class AICLIZettelMergePlugin extends Plugin {
  async onload() {
    this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
    this.client = new AICLIClient(this);
    this.addRibbonIcon("git-merge", "Run AICLI Inbox Merge", () => this.runInboxMerge());
    this.addCommand({
      id: "run-aicli-inbox-merge",
      name: "Run AICLI Inbox Merge",
      callback: () => this.runInboxMerge()
    });
    this.addCommand({
      id: "build-aicli-zettel-index",
      name: "Build AICLI Zettel Index",
      callback: () => this.buildIndex()
    });
    this.addCommand({
      id: "rollback-latest-aicli-inbox-merge",
      name: "Rollback Latest AICLI Inbox Merge",
      callback: () => this.rollbackLatest()
    });
    this.addCommand({
      id: "generate-aicli-lecture-active-note",
      name: "Generate AICLI Lecture from Active Note",
      callback: () => this.generateLectureFromActiveNote()
    });
    this.addCommand({
      id: "generate-aicli-lecture-active-folder",
      name: "Generate AICLI Lecture from Active Folder",
      callback: () => this.generateLectureFromActiveFolder()
    });
    this.addSettingTab(new AICLIZettelMergeSettingTab(this.app, this));
  }

  async saveSettings() {
    await this.saveData(this.settings);
  }

  async buildIndex() {
    await this.run("Building zettel embedding index", "/api/workflows/zettel/index", (result) => {
      new Notice(`Index ready: ${result.updated || 0} updated, ${result.reused || 0} reused.`, 8000);
    });
  }

  async runInboxMerge() {
    await this.run("Running inbox merge", "/api/workflows/zettel/inbox-merge", (result) => {
      const processed = result.processed_count || 0;
      const pending = result.pending_count || 0;
      const failed = result.failed_count || 0;
      new Notice(`Inbox merge done: ${processed} processed, ${pending} pending, ${failed} failed.`, 10000);
    });
  }

  async rollbackLatest() {
    await this.run("Rolling back latest inbox merge", "/api/workflows/zettel/rollback", (result) => {
      new Notice(`Rolled back zettel merge job ${result.job_id}.`, 10000);
    });
  }

  async generateLectureFromActiveNote() {
    const file = this.app.workspace.getActiveFile();
    if (!file || file.extension !== "md") {
      new Notice("Open a Markdown note first.", 6000);
      return;
    }
    await this.runLecture(file.path, file.basename || "Obsidian Lecture");
  }

  async generateLectureFromActiveFolder() {
    const file = this.app.workspace.getActiveFile();
    const folder = file?.parent?.path || this.settings.rootFolder || "";
    if (!folder) {
      new Notice("Open a note inside the folder you want to convert.", 6000);
      return;
    }
    await this.runLecture(folder, folder.split("/").filter(Boolean).pop() || "Obsidian Lecture");
  }

  async runLecture(sourcePath, title) {
    try {
      new Notice(`Generating lecture for ${sourcePath}...`, 5000);
      const result = await this.client.runWorkflow("/api/workflows/study/lecture", this.lecturePayload(sourcePath, title), (job) => {
        if (job.stage) new Notice(job.stage, 2500);
      });
      const script = result.script_url ? `${this.settings.baseUrl.replace(/\/+$/, "")}${result.script_url}` : "";
      const audio = result.audio_url ? `${this.settings.baseUrl.replace(/\/+$/, "")}${result.audio_url}` : "";
      const lines = [`Lecture ready: ${result.title || title}`];
      if (script) lines.push(`Script: ${script}`);
      if (audio) lines.push(`Audio: ${audio}`);
      new Notice(lines.join("\n"), 15000);
    } catch (error) {
      new Notice(`Lecture generation failed: ${error.message}`, 12000);
    }
  }

  async run(label, endpoint, onDone) {
    try {
      new Notice(`${label}...`, 4000);
      const result = await this.client.runWorkflow(endpoint, this.basePayload(), (job) => {
        if (job.stage) new Notice(job.stage, 2500);
      });
      onDone(result);
    } catch (error) {
      new Notice(`${label} failed: ${error.message}`, 10000);
    }
  }

  basePayload() {
    return {
      vault_path: this.vaultPath(),
      root_folder: this.settings.rootFolder,
      inbox_folder: this.settings.inboxFolder,
      data_folder: this.settings.dataFolder,
      shorthand_prompt_path: this.settings.shorthandPromptPath,
      provider_id: this.settings.providerId,
      merge_provider_id: this.settings.providerId,
      embedding_provider_id: this.settings.embeddingProviderId,
      merge_model: this.settings.mergeModel,
      embedding_model: this.settings.embeddingModel,
      candidate_limit: Number(this.settings.candidateLimit) || DEFAULT_SETTINGS.candidateLimit,
      embedding_batch_size: Number(this.settings.embeddingBatchSize) || DEFAULT_SETTINGS.embeddingBatchSize,
      embedding_workers: Number(this.settings.embeddingWorkers) || DEFAULT_SETTINGS.embeddingWorkers,
      inbox_workers: Number(this.settings.inboxWorkers) || DEFAULT_SETTINGS.inboxWorkers,
      inbox_random: Boolean(this.settings.inboxRandom)
    };
  }

  lecturePayload(sourcePath, title) {
    return {
      provider_id: this.settings.lectureProviderId || this.settings.providerId,
      model: this.settings.lectureModel || this.settings.mergeModel,
      vault_path: this.vaultPath(),
      source_path: sourcePath,
      output_name: title,
      style: this.settings.lectureStyle,
      max_notes: Number(this.settings.lectureMaxNotes) || DEFAULT_SETTINGS.lectureMaxNotes,
      max_input_chars: Number(this.settings.lectureMaxInputChars) || DEFAULT_SETTINGS.lectureMaxInputChars,
      synthesize_audio: Boolean(this.settings.lectureSynthesizeAudio),
      tts_command: this.settings.lectureTTSCommand,
      tts_args: this.settings.lectureTTSArgs
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
      if (current.status === "completed") return current.output ? JSON.parse(current.output) : {};
      if (current.status === "failed") throw new Error(current.error || "workflow failed");
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
    containerEl.createEl("h2", { text: "AICLI Inbox Merge" });
    this.text("AICLI URL", "Local aicli server URL.", "baseUrl");
    this.text("Vault path", "Leave empty to use the current desktop vault path.", "vaultPath");
    this.text("Source inbox folder", "Vault-relative folder containing new atomic notes.", "inboxFolder");
    this.text("Destination notes folder", "Vault-relative zettelkasten folder receiving final notes.", "rootFolder");
    this.text("Data folder", "Vault-relative archive/cache folder used by aicli.", "dataFolder");
    this.text("AI merge provider ID", "AICLI provider id for the final-note merge call.", "providerId");
    this.text("AI merge model", "Model that chooses candidate targets and returns final notes.", "mergeModel");
    this.text("Embedding provider ID", "AICLI provider id used for note similarity embeddings.", "embeddingProviderId");
    this.text("Embedding model", "Model used for semantic search.", "embeddingModel");
    this.number("Candidate limit", "Number of semantic matches sent to the merge model.", "candidateLimit");
    this.number("Parallel inbox calls", "Number of inbox notes to merge at once.", "inboxWorkers");
    this.toggle("Random inbox notes", "Pick random inbox notes when using a run limit.", "inboxRandom");
    this.number("Embedding batch size", "Notes per embedding batch while building the index.", "embeddingBatchSize");
    this.number("Embedding workers", "Parallel embedding workers while building the index.", "embeddingWorkers");
    this.text("Prompt file", "Vault-relative prompt style file, or builtin.", "shorthandPromptPath");
    containerEl.createEl("h3", { text: "Lecture generation" });
    this.text("Lecture provider ID", "AICLI provider id for notes-to-lecture generation.", "lectureProviderId");
    this.text("Lecture model", "Local model used to write the lecture script.", "lectureModel");
    this.text("Lecture style", "Prompt style for the spoken lecture.", "lectureStyle");
    this.number("Lecture max notes", "Maximum notes included from a folder.", "lectureMaxNotes");
    this.number("Lecture max input characters", "Maximum characters sent to the lecture model.", "lectureMaxInputChars");
    this.toggle("Generate lecture audio", "Use ots.TTS SOAR after script generation.", "lectureSynthesizeAudio");
    this.text("TTS command", "Command or absolute path for ots.TTS.", "lectureTTSCommand");
    this.text("TTS args", "Argument template. Supports {script}, {audio}, and {voice}.", "lectureTTSArgs");
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
          this.plugin.settings[key] = Number(value);
          await this.plugin.saveSettings();
        }));
  }

  toggle(name, desc, key) {
    new Setting(this.containerEl)
      .setName(name)
      .setDesc(desc)
      .addToggle((toggle) => toggle
        .setValue(Boolean(this.plugin.settings[key]))
        .onChange(async (value) => {
          this.plugin.settings[key] = value;
          await this.plugin.saveSettings();
        }));
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
