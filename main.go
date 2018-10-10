package main

import (
	"errors"
	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/ielab/searchrefiner"
	"net/http"
	"os"
)

type IUserStudyPlugin struct{}

type templating struct {
	UID      string
	Language string
}

const (
	bucketProgress = "progress"
)

var (
	db *bolt.DB
)

func (IUserStudyPlugin) Serve(s searchrefiner.Server, c *gin.Context) {
	uid := c.Query("uid")
	if len(uid) == 0 {
		err := errors.New("unauthorised participant in user study")
		c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	// Configure the database.
	if db == nil {
		var err error
		db, err = bolt.Open("plugin/iuserstudy/data.db", os.ModePerm, nil)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
	}

	err := db.View(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketProgress))
		if err != nil {
			return err
		}
		b.Put()
	})
	if err != nil {
		c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	// Respond to a regular request.
	c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/index.html"), templating{
		UID:      uid,
		Language: "pubmed",
	}))
}

func (IUserStudyPlugin) PermissionType() searchrefiner.PluginPermission {
	return searchrefiner.PluginPublic
}

func (IUserStudyPlugin) Details() searchrefiner.PluginDetails {
	return searchrefiner.PluginDetails{
		Title:       "searchrefiner User Study Experiment",
		Description: "Interface for participants in the searchrefiner User Study",
		Author:      "Harry Scells",
		Version:     "10.Oct.2018",
		ProjectURL:  "https://ielab.io/searchrefiner",
	}
}

var Iuserstudy = IUserStudyPlugin{}
