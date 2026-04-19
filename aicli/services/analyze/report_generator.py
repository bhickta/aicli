"""Step 7: Generate final markdown and JSON reports from aggregated data.

Combines all dimension aggregations into a single human-readable report
and a machine-readable JSON file.
"""
import json
from datetime import datetime, timezone
from pathlib import Path

from aicli.domains.analyze.database import AnalyzeDB


class ReportGeneratorService:
    """Generate final markdown and JSON reports from aggregated data."""

    def generate(self, db: AnalyzeDB, output_dir: Path) -> tuple[Path, Path]:
        """Generate techniques_report.md and techniques_report.json.

        Args:
            db: Database instance with completed aggregations.
            output_dir: Directory to write report files.

        Returns:
            (markdown_path, json_path)
        """
        output_dir.mkdir(parents=True, exist_ok=True)
        aggregations = db.get_all_aggregations()
        status = db.get_processing_status()

        # Build structured data
        report_data = {
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "summary": {
                "total_pdfs": status.get("total_pdfs", 0),
                "total_pages": status.get("total_pages", 0),
                "total_answers": status.get("total_answers", 0),
                "dimensions_analyzed": list(status.get("dimensions", {}).keys()),
            },
            "dimensions": {},
        }

        for agg in aggregations:
            dim_name = agg["dimension_name"]
            try:
                agg_data = json.loads(agg["aggregation_json"])
            except (json.JSONDecodeError, TypeError):
                agg_data = {"error": "Failed to parse aggregation JSON"}

            report_data["dimensions"][dim_name] = {
                "answer_count": agg["answer_count"],
                "generated_at": agg["generated_at"],
                "data": agg_data,
            }

        # Write JSON report
        json_path = output_dir / "techniques_report.json"
        with open(json_path, "w", encoding="utf-8") as f:
            json.dump(report_data, f, indent=2, ensure_ascii=False)

        # Build Markdown report
        md_lines = self._build_markdown(report_data, status)
        md_path = output_dir / "techniques_report.md"
        with open(md_path, "w", encoding="utf-8") as f:
            f.write("\n".join(md_lines))

        return md_path, json_path

    def _build_markdown(self, report_data: dict, status: dict) -> list[str]:
        """Build the markdown report lines."""
        lines = []

        # Header
        lines.append("# UPSC Topper Answer Sheet — Technique Analysis Report")
        lines.append("")
        lines.append(f"*Generated: {report_data['generated_at']}*")
        lines.append("")

        # Summary
        summary = report_data["summary"]
        lines.append("## Summary")
        lines.append("")
        lines.append(f"- **PDFs Analyzed**: {summary['total_pdfs']}")
        lines.append(f"- **Total Pages**: {summary['total_pages']}")
        lines.append(f"- **Answer Units Extracted**: {summary['total_answers']}")
        lines.append(f"- **Dimensions Analyzed**: {', '.join(summary['dimensions_analyzed'])}")
        lines.append("")

        # Errors summary
        errors = status.get("errors", {})
        if errors:
            lines.append("### Processing Errors")
            lines.append("")
            for step, count in errors.items():
                lines.append(f"- {step}: {count} errors")
            lines.append("")

        lines.append("---")
        lines.append("")

        # Per-dimension sections
        for dim_name, dim_info in report_data["dimensions"].items():
            lines.extend(self._build_dimension_section(dim_name, dim_info))

        return lines

    def _build_dimension_section(self, dim_name: str, dim_info: dict) -> list[str]:
        """Build markdown for a single dimension."""
        lines = []
        data = dim_info.get("data", {})

        lines.append(f"## {dim_name.upper()}")
        lines.append("")
        lines.append(f"*Analyzed {dim_info['answer_count']} answers*")
        lines.append("")

        # Patterns
        patterns = data.get("patterns", [])
        if patterns:
            lines.append("### Patterns Found")
            lines.append("")

            for i, pattern in enumerate(patterns, 1):
                name = pattern.get("pattern_name", f"Pattern {i}")
                desc = pattern.get("description", "")
                freq = pattern.get("frequency", 0)
                pct = pattern.get("percentage", 0)
                template = pattern.get("reusable_template", "")
                when_use = pattern.get("when_to_use", "")
                when_not = pattern.get("when_NOT_to_use", "")

                lines.append(f"#### {i}. {name}")
                lines.append("")
                lines.append(f"{desc}")
                lines.append("")
                lines.append(f"- **Frequency**: {freq} answers ({pct:.1f}%)")
                lines.append("")

                if template:
                    lines.append("**Reusable Template:**")
                    lines.append("```")
                    lines.append(template)
                    lines.append("```")
                    lines.append("")

                # Examples
                examples = pattern.get("examples", [])
                if examples:
                    lines.append("**Examples:**")
                    lines.append("")
                    for ex in examples[:5]:  # Cap at 5
                        candidate = ex.get("candidate", "?")
                        directive = ex.get("question_directive", "")
                        text = ex.get("exact_text", "")
                        why = ex.get("what_makes_it_work", "")
                        lines.append(f"> **{candidate}** ({directive})")
                        lines.append(f"> *\"{text}\"*")
                        if why:
                            lines.append(f"> → {why}")
                        lines.append("")

                if when_use:
                    lines.append(f"✅ **When to use**: {when_use}")
                    lines.append("")
                if when_not:
                    lines.append(f"❌ **When NOT to use**: {when_not}")
                    lines.append("")

                lines.append("---")
                lines.append("")

        # Anti-patterns
        anti = data.get("anti_patterns", [])
        if anti:
            lines.append("### Anti-Patterns (What to Avoid)")
            lines.append("")
            for ap in anti:
                pattern_text = ap.get("pattern", "")
                why = ap.get("why_avoid", "")
                seen = ap.get("seen_in", 0)
                lines.append(f"- ⚠️ **{pattern_text}** — {why} (seen in {seen} answers)")
            lines.append("")

        # Actionable rules
        rules = data.get("actionable_rules", [])
        if rules:
            lines.append("### Actionable Rules")
            lines.append("")
            for r in rules:
                rule = r.get("rule", "")
                evidence = r.get("evidence", "")
                template = r.get("template", "")
                lines.append(f"- **{rule}**")
                if evidence:
                    lines.append(f"  - Evidence: {evidence}")
                if template:
                    lines.append(f"  - Template: `{template}`")
            lines.append("")

        lines.append("")
        return lines
