package models

import (
	"fmt"

	"ricrac/ent"
)

// The short forms every answer shows referenced things in (migrations/0002_refs.sql and
// 0007_integrations.sql made them in SQL). Each is nil for a missing thing.

func UserRefOf(u *ent.User) *UserRef {
	if u == nil {
		return nil
	}
	return &UserRef{ID: ID(u.ID), Name: u.Name, DisplayName: u.DisplayName, Email: u.Email}
}

func TeamRefOf(t *ent.Team) *TeamRef {
	if t == nil {
		return nil
	}
	return &TeamRef{ID: ID(t.ID), Key: t.Key, Name: t.Name}
}

func StateRefOf(s *ent.WorkflowState) *StateRef {
	if s == nil {
		return nil
	}
	return &StateRef{ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color}
}

// Identifier is an issue's identifier, like ENG-123.
func Identifier(teamKey string, number int) string {
	return fmt.Sprintf("%s-%d", teamKey, number)
}

// IssueRefOf needs the issue's team and state loaded.
func IssueRefOf(i *ent.Issue) *IssueRef {
	if i == nil {
		return nil
	}
	return &IssueRef{ID: ID(i.ID), Identifier: Identifier(i.Edges.Team.Key, i.Number), Title: i.Title, State: *StateRefOf(i.Edges.State)}
}

// LabelRefOf needs the label's parent loaded.
func LabelRefOf(l *ent.IssueLabel) LabelRef {
	ref := LabelRef{ID: ID(l.ID), Name: l.Name, Color: l.Color}
	if l.Edges.Parent != nil {
		ref.Parent = &l.Edges.Parent.Name
	}
	return ref
}

func ProjectRefOf(p *ent.Project) *ProjectRef {
	if p == nil {
		return nil
	}
	return &ProjectRef{ID: ID(p.ID), Name: p.Name, SlugID: p.SlugID}
}

func ProjectStatusRefOf(s *ent.ProjectStatus) *ProjectStatusRef {
	if s == nil {
		return nil
	}
	return &ProjectStatusRef{ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color}
}

// ProjectLabelRefOf needs the label's parent loaded.
func ProjectLabelRefOf(l *ent.ProjectLabel) ProjectLabelRef {
	ref := ProjectLabelRef{ID: ID(l.ID), Name: l.Name, Color: l.Color}
	if l.Edges.Parent != nil {
		ref.Parent = &l.Edges.Parent.Name
	}
	return ref
}

func MilestoneRefOf(m *ent.ProjectMilestone) *MilestoneRef {
	if m == nil {
		return nil
	}
	return &MilestoneRef{ID: ID(m.ID), Name: m.Name, TargetDate: DatePtr(m.TargetDate)}
}

func CycleRefOf(c *ent.Cycle) *CycleRef {
	if c == nil {
		return nil
	}
	return &CycleRef{ID: ID(c.ID), Number: c.Number, Name: c.Name, StartsAt: Time(c.StartsAt), EndsAt: Time(c.EndsAt)}
}

func InitiativeRefOf(i *ent.Initiative) *InitiativeRef {
	if i == nil {
		return nil
	}
	return &InitiativeRef{ID: ID(i.ID), Name: i.Name, SlugID: i.SlugID}
}

func DocumentRefOf(d *ent.Document) *DocumentRef {
	if d == nil {
		return nil
	}
	return &DocumentRef{ID: ID(d.ID), Title: d.Title, SlugID: d.SlugID}
}

func ReleaseRefOf(r *ent.Release) *ReleaseRef {
	if r == nil {
		return nil
	}
	return &ReleaseRef{ID: ID(r.ID), Name: r.Name, Version: r.Version, SlugID: r.SlugID}
}

func CustomerRefOf(c *ent.Customer) *CustomerRef {
	if c == nil {
		return nil
	}
	return &CustomerRef{ID: ID(c.ID), Name: c.Name, SlugID: c.SlugID}
}

func CustomerStatusRefOf(s *ent.CustomerStatus) *CustomerStatusRef {
	if s == nil {
		return nil
	}
	return &CustomerStatusRef{ID: ID(s.ID), Name: s.Name, Color: s.Color}
}

func CustomerTierRefOf(t *ent.CustomerTier) *CustomerTierRef {
	if t == nil {
		return nil
	}
	return &CustomerTierRef{ID: ID(t.ID), Name: t.Name, Color: t.Color}
}

func ReleasePipelineRefOf(p *ent.ReleasePipeline) *ReleasePipelineRef {
	if p == nil {
		return nil
	}
	return &ReleasePipelineRef{ID: ID(p.ID), Name: p.Name, SlugID: p.SlugID}
}

func ReleaseStageRefOf(s *ent.ReleaseStage) *ReleaseStageRef {
	if s == nil {
		return nil
	}
	return &ReleaseStageRef{ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color}
}
