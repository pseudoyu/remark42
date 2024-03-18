package migrator

import (
	"encoding/json"
	"fmt"
	"github.com/umputun/remark42/backend/app/store"
	"io"
	"strings"
	"time"

	log "github.com/go-pkgz/lgr"
)

// Cusdis implements Importer from cusdis export json
type Cusdis struct {
	DataStore Store
}

type cusdisExport []struct {
	ID          string      `json:"id"`
	PageID      string      `json:"pageId"`
	CreatedAt   customTime  `json:"created_at"`
	UpdatedAt   customTime  `json:"updated_at"`
	DeletedAt   *customTime `json:"deletedAt"`
	ModeratorID *string     `json:"moderatorId"`
	ByEmail     *string     `json:"by_email"`
	ByNickname  string      `json:"by_nickname"`
	Content     string      `json:"content"`
	Approved    bool        `json:"approved"`
	ParentID    *string     `json:"parentId"`
	URL         string      `json:"url"`
}

type customTime time.Time

func (ct *customTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	layouts := []string{
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05.999999999Z",
		// Add more time formats if needed
	}

	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.UTC)
		if err == nil {
			*ct = customTime(t)
			return nil
		}
	}

	return fmt.Errorf("failed to parse time string: %s", s)
}

// Import comments from Cusdis and save to store
func (d *Cusdis) Import(r io.Reader, siteID string) (size int, err error) {
	if e := d.DataStore.DeleteAll(siteID); e != nil {
		return 0, e
	}

	commentsCh := d.convert(r, siteID)
	failed, passed := 0, 0
	for c := range commentsCh {
		if _, err = d.DataStore.Create(c); err != nil {
			failed++
			continue
		}
		passed++
	}

	if failed > 0 {
		err = fmt.Errorf("failed to save %d comments", failed)
		if passed == 0 {
			err = fmt.Errorf("import failed")
		}
	}

	log.Printf("[DEBUG] imported %d comments to site %s", passed, siteID)

	return passed, err
}

func (d *Cusdis) convert(r io.Reader, siteID string) (ch chan store.Comment) {
	commentsCh := make(chan store.Comment)

	decoder := json.NewDecoder(r)

	go func() {
		var exportedData cusdisExport
		err := decoder.Decode(&exportedData)
		if err != nil {
			log.Printf("[WARN] can't decode cusdis export json, %s", err.Error())
		}

		usersMap := map[string]store.User{}

		commentCount := 0
		for _, comment := range exportedData {
			usersMap[comment.ID] = store.User{
				Name: comment.ByNickname,
				ID:   "cusdis_" + store.EncodeID(comment.ID),
			}

			u, ok := usersMap[comment.ID]
			if !ok {
				continue
			}

			// Add current admin user to the moderator
			//if comment.ModeratorID != nil {
			//	u.ID = "github_xxx"
			//	u.Picture = "xxx"
			//}

			if comment.DeletedAt != nil {
				continue
			}

			if !comment.Approved {
				continue
			}

			var parentID string

			if comment.ParentID != nil {
				parentID = *comment.ParentID
			}

			commentUrl := comment.URL

			c := store.Comment{
				ID: comment.ID,
				Locator: store.Locator{
					URL:    commentUrl,
					SiteID: siteID,
				},
				User:      u,
				Text:      comment.Content,
				Timestamp: time.Time(comment.CreatedAt),
				ParentID:  parentID,
				Imported:  true,
			}

			commentCount++
			commentsCh <- c
		}

		close(commentsCh)
	}()

	return commentsCh
}
