package core

// Classic reference policies are fixed to the pinned simulator's level range.
// They are used by the standalone stat lab and explicit internal integration
// fixtures. The public simulator retains its inherited TBC profile.
const (
	classicReferenceCharacterLevel        int32 = 60
	classicReferenceDefaultBossLevelDelta int32 = 3
)
