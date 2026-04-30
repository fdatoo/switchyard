package api

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ProtoTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func GoTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
