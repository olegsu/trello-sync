package sync

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/olegsu/openc"
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
		IDShort          int         `json:"idShort"`
		Name             string      `json:"name"`
		Pos              int         `json:"pos"`
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
			ID      string `json:"id"`
			IDBoard string `json:"idBoard"`
			Name    string `json:"name"`
			Color   string `json:"color"`
			Uses    int    `json:"uses"`
		} `json:"labels"`
	}
)

// Handle - the function that will be called from the CLI with viper config
// to provide access to all flags
func (g *Handler) Handle(cnf *viper.Viper) error {
	p := build(cnf)
	e := openc.NewEngine(&openc.EngineOptions{
		Pipeline:          *p,
		WriteStateToFile:  cnf.GetString("store"),
		TaskLogsDirectory: cnf.GetString("logs"),
		Dryrun:            cnf.GetBool("dryRun"),
	})
	return e.Run()
}

func build(cnf *viper.Viper) *openc.Pipeline {
	return &openc.Pipeline{
		Metadata: openc.PipelineMetadata{
			Name: "sync-trello",
		},
		Spec: buildPipelineSpec(cnf),
	}
}

func buildPipelineSpec(cnf *viper.Viper) openc.PipelineSpec {
	return openc.PipelineSpec{
		Services: []openc.Service{
			openc.Service{
				Metadata: openc.ServiceMetadata{
					Name: "TrelloSVC",
				},
				Spec: openc.ServiceSpec{
					Location: cnf.GetString("trelloService"),
				},
			},
			openc.Service{
				Metadata: openc.ServiceMetadata{
					Name: "GoogleSVC",
				},
				Spec: openc.ServiceSpec{
					Location: cnf.GetString("googleSpreadsheetService"),
				},
			},
		},
		Tasks: []openc.Task{
			openc.Task{
				Metadata: buildTaskMetadata("Fetch Cards From Trello"),
				Condition: &openc.Condition{
					Name: "Engine started",
					Func: conditionOnEngineStarted(),
				},
				Spec: buildSpecTaskTrelloSync(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId")),
			},
			openc.Task{
				Metadata: buildTaskMetadata("Update Google Spreadsheet"),
				Condition: &openc.Condition{
					Name: "Fetch Cards From Trello Finished",
					Func: conditionTaskFinished("Fetch Cards From Trello"),
				},
				SpecFunc: buildSpecFuncGoogleRowsUpsert(cnf.GetString("googleServiceAccount"), cnf.GetString("googleSpreadsheetId")),
			},
			openc.Task{
				Metadata: buildTaskMetadata("Archive sync cards"),
				Condition: &openc.Condition{
					Name: "Upsert rows completed successfuly",
					Func: conditionTaskFinishedWithStatus("Update Google Spreadsheet", openc.TaskStatusSuccess),
				},
				SpecFunc: buildSpecFunncArchiveTrelloCards(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId")),
			},
		},
	}
}

func buildTaskMetadata(name string) openc.TaskMetadata {
	return openc.TaskMetadata{
		Name: name,
	}
}

func buildTask(name string, condition *openc.Condition, spec *openc.TaskSpec) openc.Task {
	return openc.Task{
		Metadata:  buildTaskMetadata(name),
		Condition: condition,
		Spec:      *spec,
	}
}

func conditionOnEngineStarted() func(ev *openc.Event, state *openc.State) bool {
	return func(ev *openc.Event, state *openc.State) bool {
		return ev.Metadata.Name == "engine.started"
	}
}

func conditionTaskFinished(task string) func(ev *openc.Event, state *openc.State) bool {
	return func(ev *openc.Event, state *openc.State) bool {
		for _, t := range state.Tasks {
			if t.State == "finished" && t.Task == task {
				return true
			}
		}
		return false
	}
}

func conditionTaskFinishedWithStatus(task string, status string) func(ev *openc.Event, state *openc.State) bool {
	return func(ev *openc.Event, state *openc.State) bool {
		for _, t := range state.Tasks {
			if t.Status == status && t.State == "finished" && t.Task == task {
				return true
			}
		}
		return false
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

func buildSpecTaskTrelloSync(trelloAppKey string, trelloToken string, trelloBoardID string) openc.TaskSpec {
	return openc.TaskSpec{
		Service:  "TrelloSVC",
		Endpoint: "GetCards",
		Arguments: []openc.Argument{
			openc.Argument{
				Key:   "App",
				Value: trelloAppKey,
			},
			openc.Argument{
				Key:   "Token",
				Value: trelloToken,
			},
			openc.Argument{
				Key:   "Board",
				Value: trelloBoardID,
			},
		},
	}
}

func buildSpecFuncGoogleRowsUpsert(googleServiceAccount string, googleSpreadsheetID string) func(state *openc.State) (*openc.TaskSpec, error) {
	return func(state *openc.State) (*openc.TaskSpec, error) {
		output := ""
		for _, t := range state.Tasks {
			if t.Task == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(output)
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
			rows = append(rows, &Row{
				ID: fmt.Sprintf("%d", c.IDShort),
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
		data, err := json.Marshal(rows)
		if err != nil {
			return nil, err
		}
		return &openc.TaskSpec{
			Service:  "GoogleSVC",
			Endpoint: "Upsert",
			Arguments: []openc.Argument{
				openc.Argument{
					Key:   "Rows",
					Value: string(data),
				},
				openc.Argument{
					Key:   "ServiceAccount",
					Value: googleServiceAccount,
				},
				openc.Argument{
					Key:   "SpreadsheetID",
					Value: googleSpreadsheetID,
				},
			},
		}, nil
	}
}

func buildSpecFunncArchiveTrelloCards(trelloAppKey string, trelloToken string, trelloBoardID string) func(state *openc.State) (*openc.TaskSpec, error) {
	return func(state *openc.State) (*openc.TaskSpec, error) {
		output := ""
		for _, t := range state.Tasks {
			if t.Task == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(output)
		if err != nil {
			return nil, err
		}
		cardids := []string{}
		for _, card := range cards {
			if card.List.Name == "Done" {
				cardids = append(cardids, card.ID)
			}
		}
		return &openc.TaskSpec{
			Service:  "TrelloSVC",
			Endpoint: "ArchiveCard",
			Arguments: []openc.Argument{
				openc.Argument{
					Key:   "App",
					Value: trelloAppKey,
				},
				openc.Argument{
					Key:   "Token",
					Value: trelloToken,
				},
				openc.Argument{
					Key:   "Board",
					Value: trelloBoardID,
				},
				openc.Argument{
					Key:   "CardIDs",
					Value: strings.Join(cardids, ","),
				},
			},
		}, nil
	}
}
