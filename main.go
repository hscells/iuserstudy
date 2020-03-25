package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/hscells/bigbro"
	"github.com/hscells/cqr"
	"github.com/hscells/groove/combinator"
	gpipeline "github.com/hscells/groove/pipeline"
	"github.com/hscells/transmute"
	"github.com/hscells/transmute/fields"
	tpipeline "github.com/hscells/transmute/pipeline"
	"github.com/ielab/searchrefiner"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type IUserStudyPlugin struct{}

type templating struct {
	UID       string
	Language  string
	Step      int
	Protocol  int
	Interface int
	Seed      []combinator.Document
	Date      string
}

const (
	bucketProgress  = "progress"
	bucketInterface = "interface"
	bucketProtocol  = "protocol"
)

const (
	pluginStorageName  = "vis_user_study"
	participantsBucket = "participants"
	consentBucket      = "consent"
)

const (
	consentGender = iota
	consentAge
	consentEducation
	consentYears
	consentGiven
)

var (
	db        *bolt.DB
	logger, _ = bigbro.NewCSVLogger("logs/bb-query_vis_study.csv")
)

var (
	//# CD012164 ['15183623', '22817861', '27590190', '17545185', '16950434', '27814432', '20109380', '12651168', '21478141', '12618690']
	//# CD010355 ['20829341', '11673215', '28437264', '21515479', '2154154', '17484789', '15223462', '12730818']
	protocol1Seed = []combinator.Document{15183623, 22817861, 27590190, 17545185, 16950434, 27814432, 20109380, 12651168, 21478141, 12618690}
	protocol2Seed = []combinator.Document{20829341, 11673215, 28437264, 21515479, 2154154, 17484789, 15223462, 12730818}
	protocol1Date = "1946:2019/12/31"
	protocol2Date = "1946:2019/12/31"
)

func handleTree(s searchrefiner.Server, c *gin.Context, date string) {
	rawQuery := c.PostForm("query")
	lang := c.PostForm("lang")

	p := make(map[string]tpipeline.TransmutePipeline)
	p["medline"] = transmute.Medline2Cqr
	p["pubmed"] = transmute.Pubmed2Cqr

	compiler := p["medline"]
	if v, ok := p[lang]; ok {
		compiler = v
	} else {
		lang = "medline"
	}

	cq, err := compiler.Execute(rawQuery)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	repr, err := cq.Representation()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var root combinator.LogicalTree
	root, _, err = combinator.NewLogicalTree(gpipeline.NewQuery("searchrefiner", "0", repr.(cqr.CommonQueryRepresentation)), s.Entrez, searchrefiner.QueryCacher)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	t := buildTree(root.Root, s.Entrez, searchrefiner.GetSettings(s, c).Relevant...)

	username := s.Perm.UserState().Username(c.Request)
	t.NumRel = len(s.Settings[username].Relevant)
	t.NumRelRet = len(t.relevant)

	bq := cqr.NewBooleanQuery(cqr.AND, []cqr.CommonQueryRepresentation{
		root.ToCQR(),
		cqr.NewKeyword(date, fields.PublicationDate),
	})

	numRet, err := s.Entrez.RetrievalSize(bq)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	s.Queries[username] = append(s.Queries[username], searchrefiner.Query{
		Time:        time.Now(),
		QueryString: rawQuery,
		Language:    lang,
		NumRet:      int64(numRet),
		NumRelRet:   int64(t.NumRelRet),
	})

	c.JSON(200, t)
}

func (IUserStudyPlugin) Serve(s searchrefiner.Server, c *gin.Context) {
	if _, ok := c.GetQuery("bigbro"); ok {
		logger.GinEndpoint(c)
		return
	}

	// Use the storage system of searchrefiner.
	storage, ok := s.Storage[pluginStorageName]
	if !ok {
		var err error
		s.Storage[pluginStorageName], err = searchrefiner.OpenPluginStorage(pluginStorageName)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error()})
			return
		}
		err = s.Storage[pluginStorageName].CreateBucket(participantsBucket)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error()})
			return
		}
		err = s.Storage[pluginStorageName].CreateBucket(consentBucket)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error()})
			return
		}
	}

	// Obtain the username, and perform a lookup to see if they are a valid participant.
	username := s.Perm.UserState().Username(c.Request)
	var uid string
	if value, err := storage.GetValue(participantsBucket, username); err == nil && len(value) > 0 {
		uid = value
	} else if err != nil {
		c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		return
	} else {
		c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/waiting.html"), nil))
		return
	}

	if _, ok := c.GetQuery("consent"); ok {
		var gender, age, education, years, agree string
		if v, ok := c.GetPostForm("gender"); ok {
			gender = v
		} else {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: "not enough information provided", BackLink: "/"})
			return
		}
		if v, ok := c.GetPostForm("age"); ok {
			age = v
		} else {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: "not enough information provided", BackLink: "/"})
			return
		}
		if v, ok := c.GetPostForm("education"); ok {
			education = v
		} else {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: "not enough information provided", BackLink: "/"})
			return
		}
		if v, ok := c.GetPostForm("years"); ok {
			years = v
		} else {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: "not enough information provided", BackLink: "/"})
			return
		}
		if v, ok := c.GetPostForm("agree"); ok {
			agree = v
		} else {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: "You must provide consent to participate in this user study.", BackLink: "/plugin/iuserstudy"})
			return
		}

		err := storage.PutValue(consentBucket, uid, strings.Join([]string{gender, age, education, years, agree}, ","))
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			return
		}

		c.Redirect(http.StatusFound, "/plugin/iuserstudy")
		return
	}

	// Obtain the consent information, if any.
	// If consent has not been provided, redirect the user to the consent form.
	consent, err := storage.GetValue(consentBucket, uid)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		return
	}

	// Check here to see if a participant has provided consent.
	consentInfo := strings.Split(consent, ",")
	if len(consentInfo) < consentGiven {
		c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/consent.html"), templating{UID: uid}))
		return
	}

	fmt.Println(uid, "consent given")

	_, ferr := os.Stat("plugin/iuserstudy/data.db")

	if db == nil {
		// Configure the database.
		db, err = bolt.Open("plugin/iuserstudy/data.db", os.ModePerm, nil)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			return
		}
		fmt.Println(uid, "opened database")
		if os.IsNotExist(ferr) {
			fmt.Println("initialising ")
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
				return
			}
		}
	}
	// Get the current progress of the user.
	var step progress
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketProgress))
		step = b.Get([]byte(uid))
		return nil
	})
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
		return
	}

	// Initialise the participant with default values and commit them to the database.
	if len(step) == 0 {
		p, err := newProgress(db)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			return
		}
		err = p.Update([]byte(uid))
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
			return
		}
		step = p
	}

	stat, i, p := step.Get()
	var seed []combinator.Document
	var date string
	if p == p1 {
		seed = protocol1Seed
		date = protocol1Date
	} else {
		seed = protocol2Seed
		date = protocol2Date
	}

	if c.Request.Method == "POST" && c.Query("tree") == "y" {
		handleTree(s, c, date)
		return
	}

	// Send the user to the tree interface if they are completing that task.
	if _, ok := c.GetQuery("tree"); ok {
		c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/queryvis.html"), templating{
			UID:      uid,
			Language: "pubmed",
			Seed:     seed,
			Date:     date,
			Protocol: int(p),
		}))
		return
	}

	// Send the user to the PubMed interface if they are completing that task.
	if _, ok := c.GetQuery("pubmed"); ok {
		c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/pubmed.html"), templating{
			UID:      uid,
			Language: "pubmed",
			Seed:     seed,
			Date:     date,
			Protocol: int(p),
		}))
		return
	}

	// Handle what happens when the participant "completes" a step.
	if c.Request.Method == "POST" {
		stat, i, p := step.Get()
		dir := path.Join("data", fmt.Sprint(i), fmt.Sprint(p))
		// Write the participants submitted query to the disk.
		if stat == experiment1 || stat == experiment2 {
			query := c.PostForm("query")
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}
			err = ioutil.WriteFile(path.Join(dir, uid), []byte(query), os.ModePerm)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}

			if queries, ok := s.Queries[username]; ok {
				f, err := os.OpenFile(path.Join(dir, fmt.Sprintf("%s_history.csv", uid)), os.O_CREATE|os.O_WRONLY, 0664)
				if err != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
					return
				}
				w := csv.NewWriter(f)
				for _, q := range queries {
					err := w.Write([]string{q.Time.String(), q.QueryString, q.Language, strconv.Itoa(int(q.NumRet)), strconv.Itoa(len(q.Relevant)), strconv.Itoa(int(q.NumRelRet))})
					if err != nil {
						c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
						return
					}
				}
				w.Flush()
				if w.Error() != nil {
					c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: w.Error().Error(), BackLink: "/plugin/iuserstudy"})
					return
				}
			} else {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: "no history found", BackLink: "/plugin/iuserstudy"})
				return
			}

			// Remove the history from the user.
			delete(s.Queries, username)
		} else {
			err = c.Request.ParseForm()
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}

			responses := make(map[string]string)
			// Record the participants answers in storage.
			for key, value := range c.Request.Form {
				if len(value) > 0 {
					responses[key] = value[0]
				} else {
					responses[key] = "NULL"
				}
			}
			responses["time"] = time.Now().String()
			v, err := json.Marshal(responses)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}

			err = storage.PutValue("step_"+responses["step"], responses["uid"], string(v))
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}

			respDir := path.Join("data", "responses", fmt.Sprint(stat))
			err = os.MkdirAll(respDir, 0777)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}
			err = ioutil.WriteFile(path.Join(respDir, fmt.Sprintf("%s.json", uid)), v, 0664)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
				return
			}
		}

		// Update the participant's interface and protocol.
		_, err := step.Step([]byte(uid), db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/plugin/iuserstudy"})
			return
		}

		c.Redirect(http.StatusFound, fmt.Sprintf("/plugin/iuserstudy?uid=%s", uid))
		return
	}

	s.Settings[username] = searchrefiner.Settings{
		Relevant: seed,
	}

	// Respond to a regular request.
	c.Render(http.StatusOK, searchrefiner.RenderPlugin(searchrefiner.TemplatePlugin("plugin/iuserstudy/index.html"), templating{
		UID:       uid,
		Step:      int(stat),
		Interface: int(i),
		Protocol:  int(p),
		Language:  "pubmed",
		Seed:      seed,
	}))
}

func (IUserStudyPlugin) PermissionType() searchrefiner.PluginPermission {
	return searchrefiner.PluginUser
}

func (IUserStudyPlugin) Details() searchrefiner.PluginDetails {
	return searchrefiner.PluginDetails{
		Title:       "Query Visualisation User Study",
		Description: "Interface for participants in the Query Visualisation User Study",
		Author:      "Harry Scells",
		Version:     "09.August.2019",
		ProjectURL:  "https://ielab.io/searchrefiner",
	}
}

var Iuserstudy = IUserStudyPlugin{}
