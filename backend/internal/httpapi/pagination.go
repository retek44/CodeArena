package httpapi

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultLimit = 20
	maxLimit     = 100
	maxCursorLen = 256
)

var errBadCursor = errors.New("invalid cursor")

func parseLimit(q url.Values) (int, error) {
	raw := q.Get("limit")
	if raw == "" {
		return defaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxLimit {
		return 0, errors.New("limit must be an integer between 1 and 100")
	}
	return n, nil
}

// Problem cursors carry the sort key of the last problem: "p:<difficulty rank>:<id>",
// for keyset pagination on (difficulty rank, id). The rank is 0 (easy) to 2 (hard).
func encodeProblemCursor(rank, id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("p:" + strconv.Itoa(rank) + ":" + strconv.Itoa(id)))
}

func decodeProblemCursor(c string) (rank, id int, err error) {
	raw, err := decodeCursor(c, "p:")
	if err != nil {
		return 0, 0, err
	}
	rawRank, rawID, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, 0, errBadCursor
	}
	rank, err = strconv.Atoi(rawRank)
	if err != nil || rank < 0 || rank > maxDifficultyRank {
		return 0, 0, errBadCursor
	}
	id, err = strconv.Atoi(rawID)
	if err != nil || id < 1 {
		return 0, 0, errBadCursor
	}
	return rank, id, nil
}

// Submission cursors carry the sort key of the last item: "s:<unix nanos>:<uuid>",
// for keyset pagination on (created_at DESC, id DESC).
func encodeSubmissionCursor(createdAt time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte("s:" + strconv.FormatInt(createdAt.UnixNano(), 10) + ":" + id.String()))
}

func decodeSubmissionCursor(c string) (time.Time, uuid.UUID, error) {
	raw, err := decodeCursor(c, "s:")
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	nanos, idText, ok := strings.Cut(raw, ":")
	if !ok {
		return time.Time{}, uuid.Nil, errBadCursor
	}
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, errBadCursor
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return time.Time{}, uuid.Nil, errBadCursor
	}
	return time.Unix(0, n), id, nil
}

func decodeCursor(c, prefix string) (string, error) {
	if len(c) > maxCursorLen {
		return "", errBadCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", errBadCursor
	}
	rest, ok := strings.CutPrefix(string(b), prefix)
	if !ok {
		return "", errBadCursor
	}
	return rest, nil
}
