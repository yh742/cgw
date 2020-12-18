package ds

import (
	"testing"

	"gotest.tools/assert"
)

func TestCreateKey(t *testing.T) {
	ep := &EntityPair{
		Entity:   "veh",
		EntityID: "1234",
	}
	assert.Equal(t, ep.CreateKey(), "veh-1234")
}

func TestGetEntityPair(t *testing.T) {
	for _, x := range []EntityIdentifier{
		&EntityTokenRequest{
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
			Token: "token.test",
		},
		&DisconnectRequest{
			EntityPair: EntityPair{
				Entity:   "veh",
				EntityID: "1234",
			},
			ReasonCode: Reauthenticate,
			NextServer: "",
		},
	} {
		assert.Equal(t, *x.GetEntityPair(), EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		})
	}
}

func TestIsValid(t *testing.T) {
	ep := &EntityPair{
		Entity:   "veh",
		EntityID: "1234",
	}
	etr := &EntityTokenRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		Token: "token.test",
	}
	dsr := &DisconnectRequest{
		EntityPair: EntityPair{
			Entity:   "veh",
			EntityID: "1234",
		},
		ReasonCode: Reauthenticate,
		NextServer: "",
	}

	t.Run("success", func(t *testing.T) {
		for _, validChk := range []ValidityChecker{
			ep, etr, dsr,
		} {
			assert.Assert(t, validChk.IsValid())
		}
	})

	t.Run("fail", func(t *testing.T) {
		ep.Entity = ""
		etr.Token = ""
		dsr.ReasonCode = ReasonCode(5)
		for _, validChk := range []ValidityChecker{
			ep, etr, dsr,
		} {
			assert.Assert(t, !validChk.IsValid())
		}
	})
}
