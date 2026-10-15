package app

type SSHAction string

const (
	SSHPushed       SSHAction = "pushed"
	SSHNoChange     SSHAction = "same"
	SSHSkippedKey   SSHAction = "skipped"
	SSHNotAsked     SSHAction = "not asked"
	SSHKeptConflict SSHAction = "conflict"
	SSHKeptDelete   SSHAction = "deleted here"
	SSHNeedsPull    SSHAction = "pull"
	SSHCreated      SSHAction = "created"
	SSHUpdated      SSHAction = "updated"
	SSHDeleted      SSHAction = "deleted"
	SSHKeptFile     SSHAction = "kept"
	SSHNeedsPush    SSHAction = "push"
	SSHFailed       SSHAction = "failed"
)

type SSHFileResult struct {
	Rel    string
	Change SSHChange
	Action SSHAction
	Err    error
}
