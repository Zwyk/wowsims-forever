package core

// Classic reference policies are fixed to the pinned simulator's level range.
// They can be inspected by tests and the standalone baseline lab, but remain
// inactive in combat until a complete, matching runtime profile selects them.
const (
	classicReferenceCharacterLevel        int32 = 60
	classicReferenceDefaultBossLevelDelta int32 = 3
)
