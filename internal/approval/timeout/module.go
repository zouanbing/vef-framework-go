package timeout

import (
	"time"

	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/cron"
)

// Module provides the timeout scanner and cron job registration.
var Module = fx.Module(
	"vef:approval:timeout",

	fx.Provide(NewScanner),
	fx.Invoke(registerTimeoutJobs),
)

func registerTimeoutJobs(scheduler cron.Scheduler, scanner *Scanner) error {
	if _, err := scheduler.NewJob(cron.NewDurationJob(
		1*time.Minute,
		cron.WithName("approval:timeout:scan"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanTimeouts),
	)); err != nil {
		return err
	}

	if _, err := scheduler.NewJob(cron.NewDurationJob(
		5*time.Minute,
		cron.WithName("approval:timeout:pre-warning"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanPreWarnings),
	)); err != nil {
		return err
	}

	return nil
}
