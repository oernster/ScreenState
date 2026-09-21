package application

import (
	"fmt"
	"time"
)

// The ceiling the user sets (NFR-PERF-003).
//
// The policy the service is built with holds the ceiling for a user who has
// chosen none; a choice is held within the bounds the policy sets. It counts
// from the moment a restore begins, whether that restore runs at sign-in or
// because Apply was pressed, so the one setting governs both.

// Ceiling answers the ceiling the next restore will run to: the user's where
// they have chosen one, the default where they have not.
func (service *RestoreService) Ceiling() (time.Duration, error) {
	chosen, set, err := service.ceilings.Ceiling()
	if err != nil {
		return 0, err
	}
	if !set {
		return service.policy.Ceiling, nil
	}
	return service.policy.WithCeiling(chosen).Ceiling, nil
}

// SetCeiling records the ceiling the next restore runs to, held within the
// bounds NFR-PERF-003 sets rather than refused. It answers the ceiling kept.
func (service *RestoreService) SetCeiling(ceiling time.Duration) (time.Duration, error) {
	kept := service.policy.WithCeiling(ceiling).Ceiling
	if err := service.ceilings.SetCeiling(kept); err != nil {
		return 0, err
	}
	service.log.Step(fmt.Sprintf("a restore now waits at most %s for windows that have not appeared", kept))
	return kept, nil
}

// ceilingFor answers the ceiling one restore runs to. A setting that cannot be
// read is no reason not to restore: the default is used and the report says so,
// since a restore that gave up sooner or later than the user set is otherwise a
// puzzle.
func (service *RestoreService) ceilingFor(report *Report) time.Duration {
	ceiling, err := service.Ceiling()
	if err != nil {
		report.Note("the ceiling setting could not be read, so the default of %s was used: %v",
			service.policy.Ceiling, err)
		return service.policy.Ceiling
	}
	return ceiling
}
