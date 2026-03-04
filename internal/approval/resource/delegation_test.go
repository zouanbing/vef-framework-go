package resource_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DelegationResourceTestSuite tests the delegation CRUD resource via HTTP.
type DelegationResourceTestSuite struct {
	apptest.Suite

	ctx   context.Context
	db    orm.DB
	token string
}

func TestDelegationResource(t *testing.T) {
	suite.Run(t, new(DelegationResourceTestSuite))
}

func (s *DelegationResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, s.token = setupResourceApp(&s.Suite)
}

func (s *DelegationResourceTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *DelegationResourceTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.Delegation)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
}

func (s *DelegationResourceTestSuite) TestCreateDelegation() {
	now := time.Now()
	end := now.Add(24 * time.Hour)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"delegatorId": "user-1",
			"delegateeId": "user-2",
			"startTime":   now.Format(time.RFC3339),
			"endTime":     end.Format(time.RFC3339),
			"isActive":    true,
			"reason":      "On vacation",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should create delegation successfully")

	// CRUD Create returns only primary keys
	data := s.ReadDataAsMap(res.Data)
	s.Assert().NotEmpty(data["id"], "Should return generated ID")
}

func (s *DelegationResourceTestSuite) TestFindAllDelegations() {
	// Insert test data
	now := timex.Now()
	end := now.Add(24 * time.Hour)
	for _, d := range []approval.Delegation{
		{DelegatorID: "user-a", DelegateeID: "user-b", IsActive: true, StartTime: now, EndTime: end},
		{DelegatorID: "user-a", DelegateeID: "user-b", IsActive: false, StartTime: now, EndTime: end},
		{DelegatorID: "user-a", DelegateeID: "user-b", IsActive: true, StartTime: now, EndTime: end},
	} {
		_, err := s.db.NewInsert().Model(&d).Exec(s.ctx)
		s.Require().NoError(err)
	}

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "find_all",
			Version:  "v1",
		},
		Meta: map[string]any{
			"delegatorId": "user-a",
			"delegateeId": "user-b",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk())

	items := s.ReadDataAsSlice(res.Data)
	s.Assert().Len(items, 3, "Should return 3 delegations")
}

func (s *DelegationResourceTestSuite) TestUpdateDelegation() {
	now := timex.Now()
	end := now.Add(24 * time.Hour)
	d := &approval.Delegation{
		DelegatorID: "user-1",
		DelegateeID: "user-2",
		IsActive:    true,
		StartTime:   now,
		EndTime:     end,
	}
	_, err := s.db.NewInsert().Model(d).Exec(s.ctx)
	s.Require().NoError(err)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":          d.ID,
			"delegatorId": "user-1",
			"delegateeId": "user-3",
			"isActive":    false,
			"startTime":   now.Format(time.RFC3339),
			"endTime":     end.Format(time.RFC3339),
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should update delegation successfully")

	// Verify update by querying DB
	var updated approval.Delegation
	updated.ID = d.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx))
	s.Assert().Equal("user-3", updated.DelegateeID)
}

func (s *DelegationResourceTestSuite) TestDeleteDelegation() {
	now := timex.Now()
	end := now.Add(24 * time.Hour)
	d := &approval.Delegation{
		DelegatorID: "user-1",
		DelegateeID: "user-2",
		IsActive:    true,
		StartTime:   now,
		EndTime:     end,
	}
	_, err := s.db.NewInsert().Model(d).Exec(s.ctx)
	s.Require().NoError(err)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/delegation",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": d.ID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should delete delegation successfully")

	// Verify deleted
	count, err := s.db.NewSelect().Model((*approval.Delegation)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("id", d.ID) }).Count(s.ctx)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count, "Delegation should be deleted")
}
