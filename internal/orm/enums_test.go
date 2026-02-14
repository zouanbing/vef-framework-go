package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinTypeString(t *testing.T) {
	tests := []struct {
		join     JoinType
		expected string
	}{
		{JoinDefault, "LEFT JOIN"},
		{JoinInner, "JOIN"},
		{JoinLeft, "LEFT JOIN"},
		{JoinRight, "RIGHT JOIN"},
		{JoinFull, "FULL JOIN"},
		{JoinCross, "CROSS JOIN"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.join.String())
	}
}

func TestFuzzyKindBuildPattern(t *testing.T) {
	tests := []struct {
		kind     FuzzyKind
		value    string
		expected string
	}{
		{FuzzyStarts, "hello", "hello%"},
		{FuzzyEnds, "hello", "%hello"},
		{FuzzyContains, "hello", "%hello%"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.kind.BuildPattern(tt.value))
	}
}

func TestNullsModeString(t *testing.T) {
	tests := []struct {
		mode     NullsMode
		expected string
	}{
		{NullsDefault, ""},
		{NullsRespect, "RESPECT NULLS"},
		{NullsIgnore, "IGNORE NULLS"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.mode.String())
	}
}

func TestFromDirectionString(t *testing.T) {
	tests := []struct {
		dir      FromDirection
		expected string
	}{
		{FromDefault, ""},
		{FromFirst, "FROM FIRST"},
		{FromLast, "FROM LAST"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.dir.String())
	}
}

func TestFrameTypeString(t *testing.T) {
	tests := []struct {
		frame    FrameType
		expected string
	}{
		{FrameDefault, ""},
		{FrameRows, "ROWS"},
		{FrameRange, "RANGE"},
		{FrameGroups, "GROUPS"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.frame.String())
	}
}

func TestFrameBoundKindString(t *testing.T) {
	tests := []struct {
		bound    FrameBoundKind
		expected string
	}{
		{FrameBoundNone, ""},
		{FrameBoundUnboundedPreceding, "UNBOUNDED PRECEDING"},
		{FrameBoundUnboundedFollowing, "UNBOUNDED FOLLOWING"},
		{FrameBoundCurrentRow, "CURRENT ROW"},
		{FrameBoundPreceding, "PRECEDING"},
		{FrameBoundFollowing, "FOLLOWING"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.bound.String())
	}
}

func TestStatisticalModeString(t *testing.T) {
	tests := []struct {
		mode     StatisticalMode
		expected string
	}{
		{StatisticalDefault, ""},
		{StatisticalPopulation, "POP"},
		{StatisticalSample, "SAMP"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.mode.String())
	}
}

func TestConflictActionString(t *testing.T) {
	tests := []struct {
		action   ConflictAction
		expected string
	}{
		{ConflictDoNothing, "DO NOTHING"},
		{ConflictDoUpdate, "DO UPDATE"},
		{ConflictAction(99), ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.action.String())
	}
}

func TestDateTimeUnitString(t *testing.T) {
	tests := []struct {
		unit     DateTimeUnit
		expected string
	}{
		{UnitYear, "YEAR"},
		{UnitMonth, "MONTH"},
		{UnitDay, "DAY"},
		{UnitHour, "HOUR"},
		{UnitMinute, "MINUTE"},
		{UnitSecond, "SECOND"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.unit.String())
	}
}

func TestDateTimeUnitForSQLite(t *testing.T) {
	tests := []struct {
		unit     DateTimeUnit
		expected string
	}{
		{UnitYear, "years"},
		{UnitMonth, "months"},
		{UnitDay, "days"},
		{UnitHour, "hours"},
		{UnitMinute, "minutes"},
		{UnitSecond, "seconds"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.unit.ForSQLite())
	}
}

func TestDateTimeUnitForDateTrunc(t *testing.T) {
	tests := []struct {
		unit     DateTimeUnit
		expected string
	}{
		{UnitYear, "year"},
		{UnitMonth, "month"},
		{UnitDay, "day"},
		{UnitHour, "hour"},
		{UnitMinute, "minute"},
		{UnitSecond, "second"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.unit.ForDateTrunc())
	}
}

func TestDateTimeUnitForPostgresAndMySQL(t *testing.T) {
	// ForPostgres and ForMySQL both delegate to String()
	unit := UnitYear
	assert.Equal(t, unit.String(), unit.ForPostgres())
	assert.Equal(t, unit.String(), unit.ForMySQL())
}
