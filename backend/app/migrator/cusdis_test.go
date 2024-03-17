package migrator

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/umputun/remark42/backend/app/store"
	"github.com/umputun/remark42/backend/app/store/admin"
	"github.com/umputun/remark42/backend/app/store/engine"
	"github.com/umputun/remark42/backend/app/store/service"
)

func TestCusdis_Import(t *testing.T) {
	defer os.Remove("/tmp/remark-test.db")
	b, err := engine.NewBoltDB(bolt.Options{}, engine.BoltSite{FileName: "/tmp/remark-test.db", SiteID: "test"})
	require.NoError(t, err, "create store")
	dataStore := service.DataStore{Engine: b, AdminStore: admin.NewStaticStore("12345", nil, []string{}, "")}
	defer dataStore.Close()

	d := Cusdis{DataStore: &dataStore}
	fh, err := os.Open("testdata/cusdis.json")
	require.NoError(t, err)
	size, err := d.Import(fh, "test")
	assert.NoError(t, err)
	assert.Equal(t, 3, size)

	last, err := dataStore.Last("test", 10, time.Time{}, adminUser)
	assert.NoError(t, err)
	require.Equal(t, 3, len(last), "10 comments imported")

	t.Log(last[0])

	c := last[0] // last reverses, get first one
	assert.Equal(t, "Postgres 数据库建议还是用 Railway 好，免费额度比较多，后面数据量大了 Heroku 可能会不够用，不过流程都一样", c.Text)
	assert.Equal(t, "db1f2775-6e63-4de1-bb24-5e922c66cc32", c.ID)
	assert.Equal(t, store.Locator{SiteID: "test", URL: "https://www.pseudoyu.com/zh/2022/05/21/free_blog_analysis_using_umami_vercel_and_heroku/"}, c.Locator)
	assert.Equal(t, "pseudoyu", c.User.Name)
	assert.Equal(t, "cusdis_debe55e5fe3788bf1f20a4f770419f9e9e2c7694", c.User.ID)
	assert.True(t, c.Imported)

	posts, err := dataStore.List("test", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(posts), "3 post")

	count, err := dataStore.Count(store.Locator{SiteID: "test", URL: "https://www.pseudoyu.com/zh/2022/05/21/free_blog_analysis_using_umami_vercel_and_heroku/"})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
