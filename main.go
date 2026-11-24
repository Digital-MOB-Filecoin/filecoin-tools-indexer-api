package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/ipfs/go-cid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/multiformats/go-multihash"
)

func main() {
	dbPath := "data.sqlite"
	if v := os.Getenv("DB_PATH"); v != "" {
		dbPath = v
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sectors/by-piece/{piece_cid}", handleSectorsByPieceCID(db))
	mux.HandleFunc("GET /providers/{provider_id}/sectors", handleSectorsByProvider(db))
	mux.HandleFunc("GET /providers/{provider_id}/sectors/{sector_id}", handleSectorByProviderAndID(db))
	mux.HandleFunc("GET /sectors/by-piece/{piece_cid}/providers/{provider_id}/{sector_id}", handleSectorByProviderIDAndPiece(db))

	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// handleSectorsByPieceCID returns all sectors that contain the given piece.
func handleSectorsByPieceCID(db *sql.DB) http.HandlerFunc {
	const query = `
		SELECT s.actor_id, s.sector_id, s.piece_id, s.state, s.bytes_power, s.afil_pledge
		FROM sectors s
		JOIN piece_digests pd ON pd.piece_id = s.piece_id
		WHERE pd.piece_digest = ?`

	return func(w http.ResponseWriter, r *http.Request) {
		digest, err := parsePieceCIDDigest(r.PathValue("piece_cid"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rows, err := db.QueryContext(r.Context(), query, digest)
		if err != nil {
			internalErr(w, "query failed", err)
			return
		}
		defer rows.Close()

		writeJSON(w, scanSectors(w, rows))
	}
}

// handleSectorsByProvider returns all sectors belonging to a provider.
func handleSectorsByProvider(db *sql.DB) http.HandlerFunc {
	const query = `
		SELECT actor_id, sector_id, piece_id, state, bytes_power, afil_pledge
		FROM sectors
		WHERE actor_id = ?`

	return func(w http.ResponseWriter, r *http.Request) {
		providerID, err := pathInt64(r, "provider_id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rows, err := db.QueryContext(r.Context(), query, providerID)
		if err != nil {
			internalErr(w, "query failed", err)
			return
		}
		defer rows.Close()

		writeJSON(w, scanSectors(w, rows))
	}
}

// handleSectorByProviderAndID returns the single sector identified by provider + sector id.
func handleSectorByProviderAndID(db *sql.DB) http.HandlerFunc {
	const query = `
		SELECT actor_id, sector_id, piece_id, state, bytes_power, afil_pledge
		FROM sectors
		WHERE actor_id = ? AND sector_id = ?`

	return func(w http.ResponseWriter, r *http.Request) {
		providerID, err := pathInt64(r, "provider_id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sectorID, err := pathInt64(r, "sector_id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var s Sector
		err = db.QueryRowContext(r.Context(), query, providerID, sectorID).
			Scan(&s.ActorID, &s.SectorID, &s.PieceID, &s.State, &s.BytesPower, &s.AfilPledge)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "sector not found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalErr(w, "query failed", err)
			return
		}

		writeJSON(w, s)
	}
}

// handleSectorByProviderIDAndPiece returns a sector only if it also contains the given piece.
func handleSectorByProviderIDAndPiece(db *sql.DB) http.HandlerFunc {
	const query = `
		SELECT s.actor_id, s.sector_id, s.piece_id, s.state, s.bytes_power, s.afil_pledge
		FROM sectors s
		JOIN piece_digests pd ON pd.piece_id = s.piece_id
		WHERE s.actor_id = ? AND s.sector_id = ? AND pd.piece_digest = ?`

	return func(w http.ResponseWriter, r *http.Request) {
		providerID, err := pathInt64(r, "provider_id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sectorID, err := pathInt64(r, "sector_id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		digest, err := parsePieceCIDDigest(r.PathValue("piece_cid"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var s Sector
		err = db.QueryRowContext(r.Context(), query, providerID, sectorID, digest).
			Scan(&s.ActorID, &s.SectorID, &s.PieceID, &s.State, &s.BytesPower, &s.AfilPledge)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "sector not found", http.StatusNotFound)
			return
		}
		if err != nil {
			internalErr(w, "query failed", err)
			return
		}

		writeJSON(w, s)
	}
}

// parsePieceCIDDigest decodes a CID string and returns the raw multihash digest bytes.
func parsePieceCIDDigest(cidStr string) ([]byte, error) {
	c, err := cid.Decode(cidStr)
	if err != nil {
		return nil, errors.New("invalid piece CID: " + err.Error())
	}
	decoded, err := multihash.Decode(c.Hash())
	if err != nil {
		return nil, errors.New("cannot decode multihash: " + err.Error())
	}
	return decoded.Digest, nil
}

// pathInt64 parses a named path value as int64.
func pathInt64(r *http.Request, key string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil {
		return 0, errors.New("invalid " + key + ": must be an integer")
	}
	return v, nil
}

// scanSectors drains rows into a Sector slice. Returns nil and writes an error
// response on failure, so callers should check for nil before writing.
func scanSectors(w http.ResponseWriter, rows *sql.Rows) []Sector {
	sectors := make([]Sector, 0)
	for rows.Next() {
		var s Sector
		if err := rows.Scan(&s.ActorID, &s.SectorID, &s.PieceID, &s.State, &s.BytesPower, &s.AfilPledge); err != nil {
			internalErr(w, "scan failed", err)
			return nil
		}
		sectors = append(sectors, s)
	}
	if err := rows.Err(); err != nil {
		internalErr(w, "rows error", err)
		return nil
	}
	return sectors
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func internalErr(w http.ResponseWriter, msg string, err error) {
	http.Error(w, msg, http.StatusInternalServerError)
	log.Printf("%s: %v", msg, err)
}
