package api

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProtoTime converts zero Go times to nil protobuf timestamps.
func ProtoTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// GoTime converts nil protobuf timestamps to the zero Go time.
func GoTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
