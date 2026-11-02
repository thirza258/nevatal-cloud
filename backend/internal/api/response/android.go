package response

// AndroidLogs is the body of GET /android/instances/:id/logs.
type AndroidLogs struct {
	Logs []string `json:"logs"`
}
