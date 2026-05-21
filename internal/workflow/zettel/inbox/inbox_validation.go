package inbox

func mechanicalInboxValidation(applied bool) MergeJudge {
	if !applied {
		return MergeJudge{}
	}
	return MergeJudge{
		Verdict: "pass",
		Score:   1,
		Notes:   "Mechanical adoption: destination_after is the full source note copied into a new zettelkasten note.",
	}
}
