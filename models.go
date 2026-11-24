package main

import "encoding/json"

type ActorType struct {
	ActorTypeID    int64            `json:"actor_type_id"`
	CodeCID        string           `json:"code_cid"`
	Label          string           `json:"label"`
	SingletonState *json.RawMessage `json:"singleton_state,omitempty"`
}

type Actor struct {
	ActorID     int64  `json:"actor_id"`
	ActorTypeID int64  `json:"actor_type_id"`
	NextNonce   int64  `json:"next_nonce"`
	AfilBalance string `json:"afil_balance"`
}

type SP struct {
	ActorID        int64  `json:"actor_id"`
	Owner          int64  `json:"owner"`
	Beneficiary    int64  `json:"beneficiary"`
	Worker         int64  `json:"worker"`
	AfilPledge     string `json:"afil_pledge"`
	AfilVesting    string `json:"afil_vesting"`
	BytesRBP       string `json:"bytes_rbp"`
	BytesQAP       string `json:"bytes_qap"`
	SectorLog2Size int16  `json:"sector_log2_size"`
}

type Sector struct {
	ActorID    int64  `json:"actor_id"`
	SectorID   int64  `json:"sector_id"`
	PieceID    int64  `json:"piece_id"`
	State      int16  `json:"state"`
	BytesPower string `json:"bytes_power"`
	AfilPledge string `json:"afil_pledge"`
}

type SectorEvent struct {
	EventID  int64 `json:"event_id"`
	ActorID  int64 `json:"actor_id"`
	SectorID int64 `json:"sector_id"`
}

type PieceDigest struct {
	PieceID       int64  `json:"piece_id"`
	Digest        []byte `json:"digest"`
	PieceLog2Size int16  `json:"piece_log2_size"`
}
