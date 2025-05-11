package migrate

import "time"

type options struct {
	migrationTimeout time.Duration
}

func WithMigrationTimeout(timeout time.Duration) func(*options) {
	return func(opts *options) {
		opts.migrationTimeout = timeout
	}
}
