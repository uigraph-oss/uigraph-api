package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/timeline"
)

const timelineEventColumns = `
	id, org_id, service_id, type, title, summary, event_date,
	version, adr_number, decision_status, source_label, source_url,
	is_agent_summarized, origin, touches,
	attachment_asset_id, attachment_file_name, attachment_file_type,
	created_by, updated_by, created_at, updated_at`

func (d *DB) CreateEvent(ctx context.Context, orgID, serviceID, actorID string, in timeline.Input) (*timeline.Event, error) {
	touchesJSON, err := json.Marshal(in.Touches)
	if err != nil {
		return nil, fmt.Errorf("postgres: CreateEvent marshal touches: %w", err)
	}

	q := fmt.Sprintf(`
		INSERT INTO timeline_events (
			org_id, service_id, type, title, summary, event_date,
			version, adr_number, decision_status, source_label, source_url,
			is_agent_summarized, origin, touches,
			attachment_asset_id, attachment_file_name, attachment_file_type,
			created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'manual', $13, $14, $15, $16, $17
		)
		RETURNING %s`, timelineEventColumns)

	row := d.db.QueryRowContext(ctx, q,
		orgID, serviceID, string(in.Type), in.Title, in.Summary, in.EventDate,
		in.Version, in.ADRNumber, decisionStatusPtr(in.DecisionStatus), in.SourceLabel, in.SourceURL,
		in.IsAgentSummarized, touchesJSON,
		in.AttachmentAssetID, in.AttachmentFileName, in.AttachmentFileType,
		actorID,
	)
	e, err := scanTimelineEvent(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: CreateEvent: %w", err)
	}
	return e, nil
}

func (d *DB) UpdateEvent(ctx context.Context, orgID, serviceID, eventID, actorID string, in timeline.Input) (*timeline.Event, error) {
	touchesJSON, err := json.Marshal(in.Touches)
	if err != nil {
		return nil, fmt.Errorf("postgres: UpdateEvent marshal touches: %w", err)
	}

	q := fmt.Sprintf(`
		UPDATE timeline_events SET
			type = $4, title = $5, summary = $6, event_date = $7,
			version = $8, adr_number = $9, decision_status = $10, source_label = $11, source_url = $12,
			is_agent_summarized = $13, touches = $14,
			attachment_asset_id = $15, attachment_file_name = $16, attachment_file_type = $17,
			updated_by = $18, updated_at = NOW()
		WHERE org_id = $1 AND service_id = $2 AND id = $3
		RETURNING %s`, timelineEventColumns)

	row := d.db.QueryRowContext(ctx, q,
		orgID, serviceID, eventID,
		string(in.Type), in.Title, in.Summary, in.EventDate,
		in.Version, in.ADRNumber, decisionStatusPtr(in.DecisionStatus), in.SourceLabel, in.SourceURL,
		in.IsAgentSummarized, touchesJSON,
		in.AttachmentAssetID, in.AttachmentFileName, in.AttachmentFileType,
		actorID,
	)
	e, err := scanTimelineEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, timeline.ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: UpdateEvent: %w", err)
	}
	return e, nil
}

func (d *DB) DeleteEvent(ctx context.Context, orgID, serviceID, eventID string) error {
	const q = `DELETE FROM timeline_events WHERE org_id = $1 AND service_id = $2 AND id = $3`
	res, err := d.db.ExecContext(ctx, q, orgID, serviceID, eventID)
	if err != nil {
		return fmt.Errorf("postgres: DeleteEvent: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return timeline.ErrEventNotFound
	}
	return nil
}

func (d *DB) GetEvent(ctx context.Context, orgID, serviceID, eventID string) (*timeline.Event, error) {
	q := fmt.Sprintf(`
		SELECT %s FROM timeline_events
		WHERE org_id = $1 AND service_id = $2 AND id = $3`, timelineEventColumns)
	e, err := scanTimelineEvent(d.db.QueryRowContext(ctx, q, orgID, serviceID, eventID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, timeline.ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetEvent: %w", err)
	}
	return e, nil
}

func (d *DB) ListEventsForService(ctx context.Context, orgID, serviceID string) ([]timeline.Event, error) {
	q := fmt.Sprintf(`
		SELECT %s FROM timeline_events
		WHERE org_id = $1 AND service_id = $2
		ORDER BY event_date DESC`, timelineEventColumns)
	rows, err := d.db.QueryContext(ctx, q, orgID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListEventsForService: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []timeline.Event
	for rows.Next() {
		e, err := scanTimelineEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListEventsForService scan: %w", err)
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func decisionStatusPtr(s *timeline.DecisionStatus) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func scanTimelineEvent(row rowScanner) (*timeline.Event, error) {
	var e timeline.Event
	var eventType, origin string
	var decisionStatus *string
	var touchesJSON []byte

	err := row.Scan(
		&e.ID, &e.OrgID, &e.ServiceID, &eventType, &e.Title, &e.Summary, &e.EventDate,
		&e.Version, &e.ADRNumber, &decisionStatus, &e.SourceLabel, &e.SourceURL,
		&e.IsAgentSummarized, &origin, &touchesJSON,
		&e.AttachmentAssetID, &e.AttachmentFileName, &e.AttachmentFileType,
		&e.CreatedBy, &e.UpdatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	e.Type = timeline.EventType(eventType)
	e.Origin = timeline.Origin(origin)
	if decisionStatus != nil {
		s := timeline.DecisionStatus(*decisionStatus)
		e.DecisionStatus = &s
	}
	e.Touches = []timeline.Touch{}
	if len(touchesJSON) > 0 {
		if err := json.Unmarshal(touchesJSON, &e.Touches); err != nil {
			return nil, err
		}
	}
	return &e, nil
}
