package sharedcontext

import (
	"strings"
)

func RenderBriefing(snapshot *Snapshot) Briefing {
	if snapshot == nil {
		return Briefing{}
	}
	lines := []string{"Task briefing:"}
	if snapshot.Summary != "" {
		lines = append(lines, "- Upstream handoff: "+snapshot.Summary)
	}
	for _, decision := range snapshot.Decisions {
		text := strings.TrimSpace(decision.Title)
		if decision.Body != "" {
			if text != "" {
				text += ": "
			}
			text += decision.Body
		}
		if text != "" {
			lines = append(lines, "- Decision: "+text)
		}
	}
	for _, issue := range snapshot.OpenIssues {
		if strings.TrimSpace(issue) != "" {
			lines = append(lines, "- Open issue: "+issue)
		}
	}
	for _, recommendation := range snapshot.Recommendations {
		if strings.TrimSpace(recommendation) != "" {
			lines = append(lines, "- Recommendation: "+recommendation)
		}
	}
	if len(snapshot.ArtifactRefs) > 0 {
		lines = append(lines, "- Artifacts to inspect: "+strings.Join(snapshot.ArtifactRefs, ", "))
	}
	return Briefing{
		SnapshotID: snapshot.SnapshotID,
		Text:       strings.Join(lines, "\n"),
	}
}
