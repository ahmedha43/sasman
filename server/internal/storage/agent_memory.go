package storage

import (
	"database/sql"
	"encoding/json"
	"time"
)

// AgentMemory holds the persistent AI memory for a single agent (router)
type AgentMemory struct {
	Subdomain             string                 `json:"subdomain"`
	RouterInfo            map[string]interface{} `json:"router_info"`
	LastAudit             map[string]interface{} `json:"last_audit"`
	AppliedCommands       []AppliedCommand       `json:"applied_commands"`
	ConversationSummaries []ConversationSummary  `json:"conversation_summaries"`
	Notes                 string                 `json:"notes"`
	UpdatedAt             string                 `json:"updated_at"`
}

// AppliedCommand records a command that was applied to the router
type AppliedCommand struct {
	Command   string `json:"command"`
	Context   string `json:"context"`
	AppliedAt string `json:"applied_at"`
}

// ConversationSummary is a compact summary of a past AI conversation session
type ConversationSummary struct {
	Summary   string `json:"summary"`
	Topic     string `json:"topic"`
	CreatedAt string `json:"created_at"`
}

// GetAgentMemory retrieves the persistent memory for a given agent subdomain.
func (r *SQLiteRepository) GetAgentMemory(subdomain string) (*AgentMemory, error) {
	row := r.db.QueryRow(
		`SELECT subdomain, router_info_json, last_audit_json,
		        applied_commands_json, conversation_summaries_json, notes, updated_at
		 FROM agent_memory WHERE subdomain = ?`, subdomain)

	var (
		sub, routerInfoRaw, lastAuditRaw string
		appliedRaw, summariesRaw         string
		notes, updatedAt                 string
	)
	err := row.Scan(&sub, &routerInfoRaw, &lastAuditRaw, &appliedRaw, &summariesRaw, &notes, &updatedAt)
	if err == sql.ErrNoRows {
		return &AgentMemory{
			Subdomain:             subdomain,
			RouterInfo:            map[string]interface{}{},
			LastAudit:             map[string]interface{}{},
			AppliedCommands:       []AppliedCommand{},
			ConversationSummaries: []ConversationSummary{},
		}, nil
	}
	if err != nil {
		return nil, err
	}

	mem := &AgentMemory{Subdomain: sub, Notes: notes, UpdatedAt: updatedAt}
	_ = json.Unmarshal([]byte(routerInfoRaw), &mem.RouterInfo)
	_ = json.Unmarshal([]byte(lastAuditRaw), &mem.LastAudit)
	_ = json.Unmarshal([]byte(appliedRaw), &mem.AppliedCommands)
	_ = json.Unmarshal([]byte(summariesRaw), &mem.ConversationSummaries)
	if mem.RouterInfo == nil { mem.RouterInfo = map[string]interface{}{} }
	if mem.LastAudit == nil { mem.LastAudit = map[string]interface{}{} }
	return mem, nil
}

// UpsertAgentMemory saves or updates the full memory for a given agent
func (r *SQLiteRepository) UpsertAgentMemory(mem *AgentMemory) error {
	routerInfoBytes, _ := json.Marshal(mem.RouterInfo)
	lastAuditBytes, _ := json.Marshal(mem.LastAudit)
	appliedBytes, _ := json.Marshal(mem.AppliedCommands)
	summariesBytes, _ := json.Marshal(mem.ConversationSummaries)
	_, err := r.db.Exec(
		`INSERT INTO agent_memory
		    (subdomain, router_info_json, last_audit_json, applied_commands_json,
		     conversation_summaries_json, notes, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(subdomain) DO UPDATE SET
		    router_info_json = excluded.router_info_json,
		    last_audit_json = excluded.last_audit_json,
		    applied_commands_json = excluded.applied_commands_json,
		    conversation_summaries_json = excluded.conversation_summaries_json,
		    notes = excluded.notes,
		    updated_at = excluded.updated_at`,
		mem.Subdomain,
		string(routerInfoBytes), string(lastAuditBytes),
		string(appliedBytes), string(summariesBytes),
		mem.Notes, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// AppendConversationSummary adds a summary and trims to maxKeep most recent
func (r *SQLiteRepository) AppendConversationSummary(subdomain, summary, topic string, maxKeep int) error {
	mem, err := r.GetAgentMemory(subdomain)
	if err != nil { return err }
	mem.ConversationSummaries = append(mem.ConversationSummaries, ConversationSummary{
		Summary: summary, Topic: topic,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if maxKeep > 0 && len(mem.ConversationSummaries) > maxKeep {
		mem.ConversationSummaries = mem.ConversationSummaries[len(mem.ConversationSummaries)-maxKeep:]
	}
	return r.UpsertAgentMemory(mem)
}

// AppendAppliedCommand records a command applied to the router (max 50)
func (r *SQLiteRepository) AppendAppliedCommand(subdomain, command, context string) error {
	mem, err := r.GetAgentMemory(subdomain)
	if err != nil { return err }
	mem.AppliedCommands = append(mem.AppliedCommands, AppliedCommand{
		Command: command, Context: context,
		AppliedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if len(mem.AppliedCommands) > 50 {
		mem.AppliedCommands = mem.AppliedCommands[len(mem.AppliedCommands)-50:]
	}
	return r.UpsertAgentMemory(mem)
}

// UpdateAgentRouterInfo merges new discovered router info into memory
func (r *SQLiteRepository) UpdateAgentRouterInfo(subdomain string, info map[string]interface{}) error {
	mem, err := r.GetAgentMemory(subdomain)
	if err != nil { return err }
	if mem.RouterInfo == nil { mem.RouterInfo = map[string]interface{}{} }
	for k, v := range info { mem.RouterInfo[k] = v }
	return r.UpsertAgentMemory(mem)
}

// UpdateAgentLastAudit stores the result of the most recent security audit
func (r *SQLiteRepository) UpdateAgentLastAudit(subdomain string, audit map[string]interface{}) error {
	mem, err := r.GetAgentMemory(subdomain)
	if err != nil { return err }
	mem.LastAudit = audit
	return r.UpsertAgentMemory(mem)
}

// UpdateAgentNotes updates the freeform notes for an agent
func (r *SQLiteRepository) UpdateAgentNotes(subdomain, notes string) error {
	mem, err := r.GetAgentMemory(subdomain)
	if err != nil { return err }
	mem.Notes = notes
	return r.UpsertAgentMemory(mem)
}

// DeleteAgentMemory completely wipes the memory for an agent
func (r *SQLiteRepository) DeleteAgentMemory(subdomain string) error {
	_, err := r.db.Exec(`DELETE FROM agent_memory WHERE subdomain = ?`, subdomain)
	return err
}