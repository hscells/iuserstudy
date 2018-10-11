package main

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/ielab/searchrefiner"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

type IUserStudyPlugin struct{}

type templating struct {
	UID       string
	Language  string
	Step      int
	Protocol  int
	Interface int
}

const (
	bucketProgress  = "progress"
	bucketInterface = "interface"
	bucketProtocol  = "protocol"
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

	// Send the user to the tree interface if they are completing that task.
	if _, ok := c.GetQuery("tree"); ok {
		c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/queryvis.html"), templating{
			UID:      uid,
			Language: "pubmed",
		}))
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
		err = db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(bucketProgress))
			if err != nil {
				return err
			}
			i, err := tx.CreateBucketIfNotExists([]byte(bucketInterface))
			if err != nil {
				return err
			}
			// check if bucket exists
			err = i.Put([]byte(bucketInterface), []byte{byte(queryvis)})
			if err != nil {
				return err
			}
			p, err := tx.CreateBucketIfNotExists([]byte(bucketProtocol))
			if err != nil {
				return err
			}
			err = p.Put([]byte(bucketProtocol), []byte{byte(p1), 0})
			return err
		})
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
	}

	// Get the current progress of the user.
	var step progress
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketProgress))
		step = b.Get([]byte(uid))
		return nil
	})
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	// Handle what happens when the participant "completes" a step.
	if c.Request.Method == "POST" {
		stat, i, p := step.Get()
		// Write the participants submitted query to the disk.
		if stat == experiment1 || stat == experiment2 {
			query := c.PostForm("query")
			dir := path.Join("data", fmt.Sprint(i), fmt.Sprint(p))
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			err = ioutil.WriteFile(path.Join(dir, uid), []byte(query), os.ModePerm)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			// Upload the history file from PubMed.
			if i == pubmed {
				fh, err := c.FormFile("history")
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				f, err := fh.Open()
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				b, err := ioutil.ReadAll(f)
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				err = ioutil.WriteFile(path.Join(dir, fmt.Sprintf("%s_history.csv", uid)), b, os.ModePerm)
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
		}

		// Update the participant's interface and protocol.
		u, err := step.Step([]byte(uid), db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		stat, i, p = u.Get()

		c.Redirect(http.StatusFound, fmt.Sprintf("/plugin/iuserstudy?uid=%s", uid))
		return
	}

	// Initialise the participant with default values and commit them to the database.
	if len(step) == 0 {
		p, err := newProgress(db)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		err = p.Update([]byte(uid))
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			c.AbortWithError(http.StatusNotFound, err)
			return
		}
		step = p
	}

	stat, i, p := step.Get()

	// Respond to a regular request.
	c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/index.html"), templating{
		UID:       uid,
		Step:      int(stat),
		Interface: int(i),
		Protocol:  int(p),
		Language:  "pubmed",
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
