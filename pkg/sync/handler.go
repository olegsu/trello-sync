package sync

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/open-integration/oi"
	"github.com/open-integration/oi/core/engine"
	"github.com/open-integration/oi/core/event"
	"github.com/open-integration/oi/core/state"
	"github.com/open-integration/oi/core/task"
	upsert "github.com/open-integration/service-catalog/google-spreadsheet/pkg/endpoints/upsert"
	"github.com/spf13/viper"
)

type (
	// Handler - exposed struct that implementd Handler interface
	Handler struct{}

	Row struct {
		ID   string   `json:"ID"`
		Data []string `json:"Data"`
	}

	TrelloCard struct {
		ID               string      `json:"id"`
		IDShort          float64     `json:"idShort"`
		Name             string      `json:"name"`
		Pos              float64     `json:"pos"`
		Email            string      `json:"email"`
		ShortLink        string      `json:"shortLink"`
		ShortURL         string      `json:"shortUrl"`
		URL              string      `json:"url"`
		Desc             string      `json:"desc"`
		Due              interface{} `json:"due"`
		DueComplete      bool        `json:"dueComplete"`
		Closed           bool        `json:"closed"`
		Subscribed       bool        `json:"subscribed"`
		DateLastActivity time.Time   `json:"dateLastActivity"`
		Board            interface{} `json:"Board"`
		IDBoard          string      `json:"idBoard"`
		List             struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			IDBoard    string `json:"idBoard"`
			Closed     bool   `json:"closed"`
			Pos        int    `json:"pos"`
			Subscribed bool   `json:"subscribed"`
		} `json:"List"`
		IDList string `json:"idList"`
		Badges struct {
			Votes              int  `json:"votes"`
			ViewingMemberVoted bool `json:"viewingMemberVoted"`
			Subscribed         bool `json:"subscribed"`
			CheckItems         int  `json:"checkItems"`
			CheckItemsChecked  int  `json:"checkItemsChecked"`
			Comments           int  `json:"comments"`
			Attachments        int  `json:"attachments"`
			Description        bool `json:"description"`
		} `json:"badges"`
		IDCheckLists          []interface{} `json:"idCheckLists"`
		IDAttachmentCover     string        `json:"idAttachmentCover"`
		ManualCoverAttachment bool          `json:"manualCoverAttachment"`
		IDLabels              []string      `json:"idLabels"`
		Labels                []struct {
			ID      string  `json:"id"`
			IDBoard string  `json:"idBoard"`
			Name    string  `json:"name"`
			Color   string  `json:"color"`
			Uses    float64 `json:"uses"`
		} `json:"labels"`
	}
)

// Handle - the function that will be called from the CLI with viper config
// to provide access to all flags
func (g *Handler) Handle(cnf *viper.Viper) error {
	p := build(cnf)
	opt := &oi.EngineOptions{
		Pipeline: *p,
	}
	if cnf.GetString("kubernetesKubeconfigPath") != "" && cnf.GetString("kubernetesNamespace") != "" && cnf.GetString("kubernetesContext") != "" {
		opt.Kubeconfig = &engine.KubernetesOptions{
			Path:      cnf.GetString("kubernetesKubeconfigPath"),
			Context:   cnf.GetString("kubernetesContext"),
			Namespace: cnf.GetString("kubernetesNamespace"),
		}
	}

	if cnf.GetBool("kubernetesInCluster") {
		namespace := "default"
		if cnf.GetString("kubernetesNamespace") != "" {
			namespace = cnf.GetString("kubernetesNamespace")
		}
		opt.Kubeconfig = &engine.KubernetesOptions{
			InCluster: true,
			Namespace: namespace,
		}
	}

	e := oi.NewEngine(opt)
	return e.Run()
}

func build(cnf *viper.Viper) *engine.Pipeline {
	return &engine.Pipeline{
		Metadata: engine.PipelineMetadata{
			Name: "sync-trello",
		},
		Spec: buildPipelineSpec(cnf),
	}
}

func buildPipelineSpec(cnf *viper.Viper) engine.PipelineSpec {
	return engine.PipelineSpec{
		Services: []engine.Service{
			{
				Name:    "trello",
				Version: "0.9.0",
				As:      "TrelloSVC",
			},
			{
				Name:    "google-spreadsheet",
				Version: "0.10.0",
				As:      "GoogleSVC",
			},
		},
		Reactions: []engine.EventReaction{
			{
				Condition: oi.ConditionEngineStarted(),
				Reaction: func(ev event.Event, state state.State) []task.Task {
					return []task.Task{
						oi.NewSerivceTask("Fetch Cards From Trello", "TrelloSVC", "getcards", buildSpecTaskTrelloSync(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId"))...),
					}
				},
			},
			{
				Condition: oi.ConditionTaskFinishedWithStatus("Fetch Cards From Trello", state.TaskStatusSuccess),
				Reaction: func(ev event.Event, state state.State) []task.Task {
					args, err := buildSpecFuncGoogleRowsUpsert(cnf.GetString("googleServiceAccount"), cnf.GetString("googleSpreadsheetId"))(state)
					if err != nil {
						return []task.Task{}
					}
					return []task.Task{
						oi.NewSerivceTask("Update Google Spreadsheet", "GoogleSVC", "upsert", args...),
					}
				},
			},
			{
				Condition: oi.ConditionTaskFinishedWithStatus("Update Google Spreadsheet", state.TaskStatusSuccess),
				Reaction: func(ev event.Event, state state.State) []task.Task {
					args, err := buildSpecFunncArchiveTrelloCards(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId"))(state)
					if err != nil {
						return []task.Task{}
					}
					return []task.Task{
						oi.NewSerivceTask("Archive sync cards", "TrelloSVC", "archivecards", args...),
					}
				},
			},
		},
	}
}

func load(j string) ([]*TrelloCard, error) {
	cards := []*TrelloCard{}
	err := json.Unmarshal([]byte(j), &cards)
	if err != nil {
		return nil, err
	}
	return cards, nil
}

func buildSpecTaskTrelloSync(trelloAppKey string, trelloToken string, trelloBoardID string) []task.Argument {
	return []task.Argument{
		{
			Key:   "App",
			Value: trelloAppKey,
		},
		{
			Key:   "Token",
			Value: trelloToken,
		},
		{
			Key:   "Board",
			Value: trelloBoardID,
		},
	}
}

func buildSpecFuncGoogleRowsUpsert(googleServiceAccount string, googleSpreadsheetID string) func(state state.State) ([]task.Argument, error) {
	f, err := ioutil.ReadFile(googleServiceAccount)
	if err != nil {
		return func(state state.State) ([]task.Argument, error) {
			return nil, err
		}
	}
	sa := &upsert.ServiceAccount{}
	err = json.Unmarshal(f, &sa)
	if err != nil {
		return func(state state.State) ([]task.Argument, error) {
			return nil, err
		}
	}
	return func(state state.State) ([]task.Argument, error) {
		output := []byte{}
		for _, t := range state.Tasks() {
			if t.Task.Name() == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(string(output))
		if err != nil {
			return nil, err
		}
		rows := []*Row{}
		for _, c := range cards {
			labels := []string{}
			for _, l := range c.Labels {
				labels = append(labels, l.Name)
			}
			now := time.Now()
			createdAt := now.AddDate(0, 0, -1).Format("02-01-2006 15:04:05")

			id := strconv.Itoa(int(c.IDShort))
			rows = append(rows, &Row{
				ID: id,
				Data: []string{
					createdAt,
					c.DateLastActivity.Format("02-01-2006 15:04:05"),
					c.Name,
					c.ShortURL,
					strings.Join(labels, " "),
					c.List.Name,
				},
			})
		}
		return []task.Argument{
			{
				Key:   "Rows",
				Value: rows,
			},
			{
				Key:   "ServiceAccount",
				Value: sa,
			},
			{
				Key:   "SpreadsheetID",
				Value: googleSpreadsheetID,
			},
		}, nil
	}
}

func buildSpecFunncArchiveTrelloCards(trelloAppKey string, trelloToken string, trelloBoardID string) func(state state.State) ([]task.Argument, error) {
	return func(state state.State) ([]task.Argument, error) {
		output := []byte{}
		for _, t := range state.Tasks() {
			if t.Task.Name() == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(string(output))
		if err != nil {
			return nil, err
		}
		cardids := []string{}
		for _, card := range cards {
			if card.List.Name == "Finished" {
				cardids = append(cardids, card.ID)
			}
		}
		return []task.Argument{
			{
				Key:   "App",
				Value: trelloAppKey,
			},
			{
				Key:   "Token",
				Value: trelloToken,
			},
			{
				Key:   "Board",
				Value: trelloBoardID,
			},
			{
				Key:   "CardIDs",
				Value: cardids,
			},
		}, nil
	}
}
