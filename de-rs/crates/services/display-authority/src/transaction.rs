#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LeasePhase {
    Testing,
    Applying,
    Pending,
    Kept,
    Reverting,
    Reverted,
    TestFailed,
    ApplyFailed,
    RollbackFailed,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LeaseEvent {
    TestSucceeded,
    TestRejected,
    ApplySucceeded,
    ApplyRejected,
    Keep,
    Revert,
    Deadline(u64),
    ClientGone,
    ServiceRestart,
    RollbackSucceeded,
    RollbackFailed,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LeaseAction {
    None,
    Apply,
    PersistConfirmed,
    Rollback,
    Complete,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct LeaseMachine {
    pub phase: LeasePhase,
    pub deadline_unix_millis: u64,
}

impl LeaseMachine {
    pub fn new(deadline_unix_millis: u64) -> Self {
        Self {
            phase: LeasePhase::Testing,
            deadline_unix_millis,
        }
    }

    pub fn transition(&mut self, event: LeaseEvent) -> LeaseAction {
        match (self.phase, event) {
            (LeasePhase::Testing, LeaseEvent::TestSucceeded) => {
                self.phase = LeasePhase::Applying;
                LeaseAction::Apply
            }
            (LeasePhase::Testing, LeaseEvent::TestRejected) => {
                self.phase = LeasePhase::TestFailed;
                LeaseAction::Complete
            }
            (LeasePhase::Applying, LeaseEvent::ApplySucceeded) => {
                self.phase = LeasePhase::Pending;
                LeaseAction::None
            }
            (LeasePhase::Applying, LeaseEvent::ApplyRejected) => {
                self.phase = LeasePhase::ApplyFailed;
                LeaseAction::Complete
            }
            (LeasePhase::Pending, LeaseEvent::Keep) => {
                self.phase = LeasePhase::Kept;
                LeaseAction::PersistConfirmed
            }
            (
                LeasePhase::Pending,
                LeaseEvent::Revert | LeaseEvent::ClientGone | LeaseEvent::ServiceRestart,
            ) => {
                self.phase = LeasePhase::Reverting;
                LeaseAction::Rollback
            }
            (LeasePhase::Pending, LeaseEvent::Deadline(now))
                if now >= self.deadline_unix_millis =>
            {
                self.phase = LeasePhase::Reverting;
                LeaseAction::Rollback
            }
            (LeasePhase::Reverting, LeaseEvent::RollbackSucceeded) => {
                self.phase = LeasePhase::Reverted;
                LeaseAction::Complete
            }
            (LeasePhase::Reverting, LeaseEvent::RollbackFailed) => {
                self.phase = LeasePhase::RollbackFailed;
                LeaseAction::Complete
            }
            _ => LeaseAction::None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keep_is_the_only_persistence_path() {
        let mut machine = LeaseMachine::new(1_000);
        assert_eq!(
            machine.transition(LeaseEvent::TestSucceeded),
            LeaseAction::Apply
        );
        machine.transition(LeaseEvent::ApplySucceeded);
        assert_eq!(
            machine.transition(LeaseEvent::Keep),
            LeaseAction::PersistConfirmed
        );
        assert_eq!(machine.phase, LeasePhase::Kept);
    }

    #[test]
    fn timeout_client_death_and_service_restart_all_require_rollback() {
        for event in [
            LeaseEvent::Deadline(1_001),
            LeaseEvent::ClientGone,
            LeaseEvent::ServiceRestart,
        ] {
            let mut machine = LeaseMachine::new(1_000);
            machine.transition(LeaseEvent::TestSucceeded);
            machine.transition(LeaseEvent::ApplySucceeded);
            assert_eq!(machine.transition(event), LeaseAction::Rollback);
            assert_eq!(machine.phase, LeasePhase::Reverting);
        }
    }

    #[test]
    fn rollback_failure_remains_distinct() {
        let mut machine = LeaseMachine::new(1_000);
        machine.transition(LeaseEvent::TestSucceeded);
        machine.transition(LeaseEvent::ApplySucceeded);
        machine.transition(LeaseEvent::Revert);
        assert_eq!(
            machine.transition(LeaseEvent::RollbackFailed),
            LeaseAction::Complete
        );
        assert_eq!(machine.phase, LeasePhase::RollbackFailed);
    }

    #[test]
    fn test_and_apply_rejection_are_distinct_terminal_states() {
        let mut test = LeaseMachine::new(1_000);
        assert_eq!(
            test.transition(LeaseEvent::TestRejected),
            LeaseAction::Complete
        );
        assert_eq!(test.phase, LeasePhase::TestFailed);

        let mut apply = LeaseMachine::new(1_000);
        apply.transition(LeaseEvent::TestSucceeded);
        assert_eq!(
            apply.transition(LeaseEvent::ApplyRejected),
            LeaseAction::Complete
        );
        assert_eq!(apply.phase, LeasePhase::ApplyFailed);
    }
}
