package lecture

import "strings"

func lecturePrompt(notes []noteInput, style string) string {
	style = strings.TrimSpace(style)
	if style == "" {
		style = "crisp comprehensive UPSC lecture"
	}
	var b strings.Builder
	b.WriteString(`You are creating a spoken lecture script from Obsidian UPSC notes.

Goal:
- Make the lecture crisp but comprehensive.
- Preserve every important concept, definition, example, framework, chronology, cause-effect link, criticism, committee, article, scheme, data point, and exam angle present in the notes.
- Remove duplicated wording and metadata noise.
- Explain in a teacher-like flow that is easy to listen to.
- Use clear section headings.
- Use short paragraphs suitable for TTS.
- Do not mention "the note says" or "source note".
- Do not invent facts not present in the notes.
- If the notes contain contradictions, mention the conflict briefly instead of silently resolving it.

Output:
- Markdown lecture script only.
- Start with a title.
- Then give a 1-minute orientation.
- Then the main lecture in logical sections.
- End with a compact recap and 5 broad recall questions.

Lecture style: `)
	b.WriteString(style)
	b.WriteString("\n\nNOTES:\n")
	for _, note := range notes {
		b.WriteString("\n\n---\nSOURCE: ")
		b.WriteString(note.Rel)
		b.WriteString("\n")
		b.WriteString(note.Content)
	}
	return b.String()
}
