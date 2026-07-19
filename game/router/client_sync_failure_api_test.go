package router

import (
	"encoding/json"
	"testing"

	pb "spider-server/gen/spider/api"

	"google.golang.org/protobuf/proto"
)

func TestReadableClientRequestJSONUsesProvidedBusinessData(t *testing.T) {
	req := &pb.ArchiveClientSyncFailureRequest{
		OriginalRpcPath:     "/queued/dailyRecord",
		OriginalRequestBody: []byte{0x01, 0x02},
		RequestDataJson:     ` { "weight": 700, "recordDate": 123 } `,
	}

	got := readableClientRequestJSON(req)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("request JSON should be valid: %v", err)
	}
	if decoded["weight"] != float64(700) {
		t.Fatalf("unexpected readable request data: %s", got)
	}
}

func TestReadableClientRequestJSONDecodesProtobufByRPCPath(t *testing.T) {
	original := &pb.SaveWeightRecordRequest{
		RecordDate: "2026-07-20",
		Weight:     700,
		Satiety:    6,
	}
	body, err := proto.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	got := readableClientRequestJSON(&pb.ArchiveClientSyncFailureRequest{
		OriginalRpcPath:     "/health.WeightRecordService/SaveWeightRecord",
		OriginalRequestBody: body,
	})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("decoded protobuf JSON should be valid: %v", err)
	}
	if decoded["record_date"] != "2026-07-20" || decoded["weight"] != float64(700) {
		t.Fatalf("unexpected protobuf request data: %s", got)
	}
}

func TestReadableClientRequestJSONKeepsUnknownBodyAsBase64(t *testing.T) {
	got := readableClientRequestJSON(&pb.ArchiveClientSyncFailureRequest{
		OriginalRpcPath:     "/unknown.Service/Write",
		OriginalRequestBody: []byte{0x01, 0x02},
	})
	if got != `{"protobuf_base64":"AQI="}` {
		t.Fatalf("unexpected fallback: %s", got)
	}
}
