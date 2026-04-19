import re
from pathlib import Path

fpath = Path("aicli/server/pipelines/analyze.py")
content = fpath.read_text()

# Add target_steps to sig
content = content.replace("llm_model: str | None = None,\n):", "llm_model: str | None = None,\n    target_steps: list[int] | None = None,\n):")

def indent_block(text):
    return "\n".join("    " + line if line.strip() else line for line in text.split("\n"))

steps = {
    1: r'(\[bold magenta\]━━━ Step 1.*?)(?=\n    # -{66}\n    # Step 2)',
    2: r'(\[bold magenta\]━━━ Step 2.*?)(?=\n    # -{66}\n    # Step 3)',
    3: r'(\[bold magenta\]━━━ Step 3.*?)(?=\n    # -{66}\n    # Step 4)',
    4: r'(\[bold magenta\]━━━ Step 4.*?)(?=\n    # -{66}\n    # Step 5)',
    5: r'(\[bold magenta\]━━━ Step 5.*?)(?=\n    # -{66}\n    # Step 6)',
    6: r'(\[bold magenta\]━━━ Step 6.*?)(?=\n    # -{66}\n    # Step 7)',
    7: r'(\[bold magenta\]━━━ Step 7.*?)(?=\n\n\ndef )',
}

for step_num, pattern in steps.items():
    match = re.search(pattern, content, re.DOTALL)
    if match:
        orig = match.group(1)
        new_block = f"if target_steps is None or {step_num} in target_steps:\n{indent_block(orig)}"
        
        # fix the start of the block to match formatting
        # since orig starts with console.print, we want:
        # if cond:
        #     console.print...
        
        content = content.replace(orig, new_block)

fpath.write_text(content)
print("Patched.")
