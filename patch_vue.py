from pathlib import Path
import re

fpath = Path("frontend/src/components/AnalyzeStudio.vue")
content = fpath.read_text()

# Fix data block
content = content.replace("""      runConfig: {
        workers: 4,
        dpi: 200,
        llm_model: 'gemma-4-26b-a4b'
      },""", """      runConfig: {
        mode: 'all',
        target_steps: [1, 2, 3, 4, 5, 6, 7],
        workers: 4,
        dpi: 200,
        llm_model: 'gemma-4-26b-a4b'
      },""")

# Fix startPipeline block
orig_start = """    async startPipeline() {
      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        await runPipeline(this.runConfig)
        this.connectStream()
      } catch (err) {"""

new_start = """    async startPipeline() {
      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const payload = {
          workers: this.runConfig.workers,
          dpi: this.runConfig.dpi,
          llm_model: this.runConfig.llm_model,
          target_steps: this.runConfig.mode === 'all' ? null : this.runConfig.target_steps
        }
        await runPipeline(payload)
        this.connectStream()
      } catch (err) {"""
content = content.replace(orig_start, new_start)

# Add CSS
if ".steps-grid" not in content:
    content = content.replace("</style>", """
.steps-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  background: var(--bg-body);
  padding: 12px;
  border-radius: var(--radius-md);
  margin-top: 8px;
  border: 1px solid var(--border);
}
.steps-grid label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-primary);
  margin: 0;
}
.span-full {
  grid-column: 1 / -1;
}
</style>""")

fpath.write_text(content)
print("Patched.")
