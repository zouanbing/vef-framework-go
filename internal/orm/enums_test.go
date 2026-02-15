package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinTypeString(t *testing.T) {
	tests := []struct {
		name     string
		join     JoinType
		expected string
	}{
		{"DefaultJoin", JoinDefault, "LEFT JOIN"},
		{"InnerJoin", JoinInner, "JOIN"},
		{"LeftJoin", JoinLeft, "LEFT JOIN"},
		{"RightJoin", JoinRight, "RIGHT JOIN"},
		{"FullJoin", JoinFull, "FULL JOIN"},
		{"CrossJoin", JoinCross, "CROSS JOIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.join.String(), "Should return correct SQL keyword")
		})
	}
}

func TestFuzzyKindBuildPattern(t *testing.T) {
	tests := []struct {
		name     string
		kind     FuzzyKind
		value    string
		expected string
	}{
		{"StartsWithPattern", FuzzyStarts, "hello", "hello%"},
		{"EndsWithPattern", FuzzyEnds, "hello", "%hello"},
		{"ContainsPattern", FuzzyContains, "hello", "%hello%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.BuildPattern(tt.value), "Should build correct LIKE pattern")
		})
	}
}

func TestNullsModeString(t *testing.T) {
	tests := []struct {
		name     string
		mode     NullsMode
		expected string
	}{
		{"DefaultMode", NullsDefault, ""},
		{"RespectNulls", NullsRespect, "RESPECT NULLS"},
		{"IgnoreNulls", NullsIgnore, "IGNORE NULLS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String(), "Should return correct nulls mode string")
		})
	}
}

func TestFromDirectionString(t *testing.T) {
	tests := []struct {
		name     string
		dir      FromDirection
		expected string
	}{
		{"DefaultDirection", FromDefault, ""},
		{"FromFirst", FromFirst, "FROM FIRST"},
		{"FromLast", FromLast, "FROM LAST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.dir.String(), "Should return correct direction string")
		})
	}
}

func TestFrameTypeString(t *testing.T) {
	tests := []struct {
		name     string
		frame    FrameType
		expected string
	}{
		{"DefaultFrame", FrameDefault, ""},
		{"RowsFrame", FrameRows, "ROWS"},
		{"RangeFrame", FrameRange, "RANGE"},
		{"GroupsFrame", FrameGroups, "GROUPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.frame.String(), "Should return correct frame type string")
		})
	}
}

func TestFrameBoundKindString(t *testing.T) {
	tests := []struct {
		name     string
		bound    FrameBoundKind
		expected string
	}{
		{"NoBound", FrameBoundNone, ""},
		{"UnboundedPreceding", FrameBoundUnboundedPreceding, "UNBOUNDED PRECEDING"},
		{"UnboundedFollowing", FrameBoundUnboundedFollowing, "UNBOUNDED FOLLOWING"},
		{"CurrentRow", FrameBoundCurrentRow, "CURRENT ROW"},
		{"Preceding", FrameBoundPreceding, "PRECEDING"},
		{"Following", FrameBoundFollowing, "FOLLOWING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.bound.String(), "Should return correct frame bound string")
		})
	}
}

func TestStatisticalModeString(t *testing.T) {
	tests := []struct {
		name     string
		mode     StatisticalMode
		expected string
	}{
		{"DefaultStatistical", StatisticalDefault, ""},
		{"PopulationMode", StatisticalPopulation, "POP"},
		{"SampleMode", StatisticalSample, "SAMP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String(), "Should return correct statistical mode string")
		})
	}
}

func TestConflictActionString(t *testing.T) {
	tests := []struct {
		name     string
		action   ConflictAction
		expected string
	}{
		{"DoNothing", ConflictDoNothing, "DO NOTHING"},
		{"DoUpdate", ConflictDoUpdate, "DO UPDATE"},
		{"UnknownAction", ConflictAction(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.String(), "Should return correct conflict action string")
		})
	}
}

func TestDateTimeUnitString(t *testing.T) {
	tests := []struct {
		name     string
		unit     DateTimeUnit
		expected string
	}{
		{"Year", UnitYear, "YEAR"},
		{"Month", UnitMonth, "MONTH"},
		{"Day", UnitDay, "DAY"},
		{"Hour", UnitHour, "HOUR"},
		{"Minute", UnitMinute, "MINUTE"},
		{"Second", UnitSecond, "SECOND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.unit.String(), "Should return correct SQL keyword")
		})
	}
}

func TestDateTimeUnitForSQLite(t *testing.T) {
	tests := []struct {
		name     string
		unit     DateTimeUnit
		expected string
	}{
		{"Year", UnitYear, "years"},
		{"Month", UnitMonth, "months"},
		{"Day", UnitDay, "days"},
		{"Hour", UnitHour, "hours"},
		{"Minute", UnitMinute, "minutes"},
		{"Second", UnitSecond, "seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.unit.ForSQLite(), "Should return correct SQLite modifier")
		})
	}
}

func TestDateTimeUnitForDateTrunc(t *testing.T) {
	tests := []struct {
		name     string
		unit     DateTimeUnit
		expected string
	}{
		{"Year", UnitYear, "year"},
		{"Month", UnitMonth, "month"},
		{"Day", UnitDay, "day"},
		{"Hour", UnitHour, "hour"},
		{"Minute", UnitMinute, "minute"},
		{"Second", UnitSecond, "second"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.unit.ForDateTrunc(), "Should return correct date_trunc field")
		})
	}
}

func TestDateTimeUnitForPostgresAndMySQL(t *testing.T) {
	unit := UnitYear
	assert.Equal(t, unit.String(), unit.ForPostgres(), "ForPostgres should delegate to String")
	assert.Equal(t, unit.String(), unit.ForMySQL(), "ForMySQL should delegate to String")
}

func TestIndexMethodString(t *testing.T) {
	tests := []struct {
		name     string
		method   IndexMethod
		expected string
	}{
		{"BTree", IndexBTree, "BTREE"},
		{"Hash", IndexHash, "HASH"},
		{"GIN", IndexGIN, "GIN"},
		{"GiST", IndexGiST, "GiST"},
		{"SPGiST", IndexSPGiST, "SPGiST"},
		{"BRIN", IndexBRIN, "BRIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.method.String(), "Should return correct index method string")
		})
	}
}

func TestPartitionStrategyString(t *testing.T) {
	tests := []struct {
		name     string
		strategy PartitionStrategy
		expected string
	}{
		{"RangePartition", PartitionRange, "RANGE"},
		{"ListPartition", PartitionList, "LIST"},
		{"HashPartition", PartitionHash, "HASH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.strategy.String(), "Should return correct partition strategy string")
		})
	}
}
