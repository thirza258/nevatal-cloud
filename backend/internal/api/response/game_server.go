package response

// CommandResult is the console reply of POST /game-servers/:id/command.
type CommandResult struct {
	Response string `json:"response"`
}
