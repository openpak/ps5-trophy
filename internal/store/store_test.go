package store

import "testing"

func TestValidate(t *testing.T) {
	s := Set{NpCommunicationID: "X", Groups: []Group{{ID: "default", Trophies: []Trophy{{ID: 0, Type: "bronze"}, {ID: 0, Type: "gold"}}}}}
	if s.Validate() == nil {
		t.Fatal("duplicate trophy id accepted")
	}
	s.Groups[0].Trophies[1].ID = 1
	if err := s.Validate(); err != nil || s.NpServiceName != "trophy2" {
		t.Fatal(err, s.NpServiceName)
	}
}
