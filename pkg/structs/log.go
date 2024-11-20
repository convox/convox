package structs

import "time"

type LogsOptions struct {
	Filter   *string        `flag:"filter" header:"Filter"`
	Follow   *bool          `header:"Follow"`
	Prefix   *bool          `header:"Prefix"`
	Since    *time.Duration `default:"2m" flag:"since" header:"Since"`
	Previous *bool          `flag:"allow-previous" header:"Previous"`
	Tail     *int           `flag:"tail" header:"Tail"`
}
