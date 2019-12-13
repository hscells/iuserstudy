package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
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
	protocol1Relevant = []combinator.Document{24355200, 18673544, 16873795, 12777437, 9705020, 25906786, 14710970, 21278140, 21676480, 21300382, 10862313, 27596059, 17315136, 6702817, 3516770, 21307378, 15220202, 20573752, 12414877, 20622160, 10466767, 11141143, 9300248, 2060716, 11679461, 12757990, 10414941, 27459384, 20073428, 17309402, 9829346, 28698884, 23283714, 26273669, 27933333, 14967156, 22647753, 27543801, 25029368, 25131451, 26606421, 18282630, 19531260, 19414206, 8826973, 2035513, 6706044, 3542644, 19224196, 14693710, 9243105, 8886564, 15983331, 28278309, 28394951, 26104243, 14567760, 17257284, 20978739, 10333943, 7712700, 27515749, 18689695, 16801588, 20484131, 21741107, 21824186, 26675051, 11686540, 17032347, 11068083, 21335372, 28043048, 29133894, 29380232, 28951335, 22640983, 8635647, 11916954, 16344402, 8680609, 22456865, 22510023, 22955996, 21212932, 9162608, 28751960, 22580730, 25814432, 27740930, 24843430, 16518992, 18486512, 16100444, 15223223, 18452257, 25245975, 27368062, 20536946, 20827664, 10587859, 11606173, 7782724, 9754834, 11079739, 28004008, 23497506, 17320447, 26885316, 25215305, 17914548, 14025561, 28768835, 29074816, 18206734, 3706388, 17533210, 9653617, 12590020, 10812323, 28258520, 26575606, 28632742, 24135387, 18535192, 18485514, 24992623, 27749572, 26840038, 7589843, 9028719, 2407581, 14578254, 8482427, 6507426, 8866565, 15616025, 12062857, 11978676, 17989310, 21824186, 23389687, 28371687, 2912042, 8112189, 7859632, 2689122, 8894485, 12610034, 12032097, 12765960, 18697630, 11437858, 8070301, 8454106, 9203444, 12519316, 9363520, 7748921, 19414203, 8335178, 1892482, 2261821, 27515716, 15036828, 17000944, 24843514, 20508383, 16600415, 20339479, 28929513, 21909836, 21738002, 8922541, 7481176, 11106838, 3527626, 18060659, 17143605, 28143481, 20693490, 11784224, 12627316, 20002472, 8612442, 8720611, 17259503, 29018885, 11311100, 28677982, 25350916, 21107436, 7075915, 19131461, 17536075, 18316395, 2752891, 24083174, 7497867, 9405904, 10097917, 15189364, 27085081, 28938752, 25962707, 25624343, 27239315, 16990660, 18226046, 3751746, 8314414, 19046200, 10480514, 17536076, 20934897, 18249214, 27810987, 18405128, 16720024, 15451912, 15533586, 11772900, 2260546, 20578203, 21270194, 29074816, 16043747, 24703046, 20855549, 20200384, 27863979, 28108537, 9406673, 11110508, 11781759, 15175438, 15793193, 26913636, 10333940, 14578234, 12397006, 21718910, 10663216, 1216390, 15161800}
	protocol1Seed     = []combinator.Document{25906786, 21278140, 21300382, 20536946, 25245975}
	protocol2Relevant = []combinator.Document{24355200, 18673544, 16873795, 12777437, 9705020, 25906786, 14710970, 21278140, 21676480, 21300382, 10862313, 27596059, 17315136, 6702817, 3516770, 21307378, 15220202, 20573752, 12414877, 20622160, 10466767, 11141143, 9300248, 2060716, 11679461, 12757990, 10414941, 27459384, 20073428, 17309402, 9829346, 28698884, 23283714, 26273669, 27933333, 14967156, 22647753, 27543801, 25029368, 25131451, 26606421, 18282630, 19531260, 19414206, 8826973, 2035513, 6706044, 3542644, 19224196, 14693710, 9243105, 8886564, 15983331, 28278309, 28394951, 26104243, 14567760, 17257284, 20978739, 10333943, 7712700, 27515749, 18689695, 16801588, 20484131, 21741107, 21824186, 26675051, 11686540, 17032347, 11068083, 21335372, 28043048, 29133894, 29380232, 28951335, 22640983, 8635647, 11916954, 16344402, 8680609, 22456865, 22510023, 22955996, 21212932, 9162608, 28751960, 22580730, 25814432, 27740930, 24843430, 16518992, 18486512, 16100444, 15223223, 18452257, 25245975, 27368062, 20536946, 20827664, 10587859, 11606173, 7782724, 9754834, 11079739, 28004008, 23497506, 17320447, 26885316, 25215305, 17914548, 14025561, 28768835, 29074816, 18206734, 3706388, 17533210, 9653617, 12590020, 10812323, 28258520, 26575606, 28632742, 24135387, 18535192, 18485514, 24992623, 27749572, 26840038, 7589843, 9028719, 2407581, 14578254, 8482427, 6507426, 8866565, 15616025, 12062857, 11978676, 17989310, 21824186, 23389687, 28371687, 2912042, 8112189, 7859632, 2689122, 8894485, 12610034, 12032097, 12765960, 18697630, 11437858, 8070301, 8454106, 9203444, 12519316, 9363520, 7748921, 19414203, 8335178, 1892482, 2261821, 27515716, 15036828, 17000944, 24843514, 20508383, 16600415, 20339479, 28929513, 21909836, 21738002, 8922541, 7481176, 11106838, 3527626, 18060659, 17143605, 28143481, 20693490, 11784224, 12627316, 20002472, 8612442, 8720611, 17259503, 29018885, 11311100, 28677982, 25350916, 21107436, 7075915, 19131461, 17536075, 18316395, 2752891, 24083174, 7497867, 9405904, 10097917, 15189364, 27085081, 28938752, 25962707, 25624343, 27239315, 16990660, 18226046, 3751746, 8314414, 19046200, 10480514, 17536076, 20934897, 18249214, 27810987, 18405128, 16720024, 15451912, 15533586, 11772900, 2260546, 20578203, 21270194, 29074816, 16043747, 24703046, 20855549, 20200384, 27863979, 28108537, 9406673, 11110508, 11781759, 15175438, 15793193, 26913636, 10333940, 14578234, 12397006, 21718910, 10663216, 1216390, 15161800}
	protocol2Seed     = []combinator.Document{25906786, 21278140, 21300382, 20536946, 25245975}
	protocol1Date     = "1946:2018/02/26"
	protocol2Date     = "1946:2018/02/26"
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
		err := errors.New("unauthorised participant in user study")
		c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
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

	// Configure the database.
	if db == nil {
		var err error
		db, err = bolt.Open("plugin/iuserstudy/data.db", os.ModePerm, nil)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "error.html", searchrefiner.ErrorPage{Error: err.Error(), BackLink: "/"})
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
			return
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
