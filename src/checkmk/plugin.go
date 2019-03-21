package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"io"
	"net/http"
	"net/url"
	"sync"
)

const (
	commandGenericError = "Something went bad. Please try again later."
	pluginId            = "au.com.slicedtech.chat.checkmk"
	infoMessage         = "Thanks for using CheckMK v1.0.0\n"
)

type CheckMKPlugin struct {
	plugin.MattermostPlugin
	router            *mux.Router
	ServerConfig      *model.Config
	configurationLock sync.RWMutex
	configuration     *configuration
}

func (p *CheckMKPlugin) OnActivate() error {
	p.router = p.InitAPI()
	return nil
}

func (p *CheckMKPlugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.API.LogDebug("New request:", "Host", r.Host, "RequestURI", r.RequestURI, "Method", r.Method)
	p.router.ServeHTTP(w, r)
}

func (p *CheckMKPlugin) InitAPI() *mux.Router {
	p.API.LogDebug("Inside InitAPI")
	r := mux.NewRouter()
	r.HandleFunc("/", p.handleInfo).Methods("GET")
	apiV1 := r.PathPrefix("/api/v1").Subrouter()
	pollRouter := apiV1.PathPrefix("/cmk/{id:[a-z0-9]+}").Subrouter()
	pollRouter.HandleFunc("/ack", p.handleCmkDialogRequest).Methods("POST")
	pollRouter.HandleFunc("/doack", p.handleCmkDialogSubmit).Methods("POST")

	return r
}

func (p *CheckMKPlugin) handleInfo(w http.ResponseWriter, _ *http.Request) {
	p.API.LogDebug("Inside Handle Info")
	_, _ = io.WriteString(w, infoMessage)
}

func (p *CheckMKPlugin) handleCmkDialogRequest(w http.ResponseWriter, r *http.Request) {
	p.API.LogDebug("Inside Handle Check_MK Dialog Request")
	cmkId := mux.Vars(r)["id"]
	response := &model.PostActionIntegrationResponse{}
	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	siteURLValue := *siteURL
	dialogUrl := fmt.Sprintf("%s/plugins/%s/api/v1/cmk/%s/doack", siteURLValue, pluginId, cmkId)
	dialog := model.OpenDialogRequest{
		TriggerId: request.TriggerId,
		URL:       dialogUrl,
		Dialog: model.Dialog{
			Title:       "Acknowledge CheckMK Alert",
			CallbackId:  request.PostId,
			SubmitLabel: "Acknowledge",
			Elements: []model.DialogElement{{
				DisplayName: "Message",
				Name:        "message",
				Type:        "text",
				SubType:     "text",
			}},
		},
	}
	if appErr := p.API.OpenInteractiveDialog(dialog); appErr != nil {
		p.API.LogError("failed to open add option dialog ", "err", appErr.Error())
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	writePostActionIntegrationResponse(w, response)
}

func (p *CheckMKPlugin) handleCmkDialogSubmit(w http.ResponseWriter, r *http.Request) {
	p.API.LogDebug("Inside Dialog Submit")
	request := model.SubmitDialogRequestFromJson(r.Body)
	if request == nil {
		p.API.LogError("failed to decode request")
		p.SendEphemeralPost(request.ChannelId, request.UserId, commandGenericError)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	theMessage, ok := request.Submission["message"].(string)
	if !ok {
		p.API.LogError("failed to parse request")
		p.SendEphemeralPost(request.ChannelId, request.UserId, commandGenericError)
		w.WriteHeader(http.StatusOK)
		return
	}

	post, appErr := p.API.GetPost(request.CallbackId)
	if appErr != nil {
		p.API.LogError("failed to get post", "err", appErr.Error())
		p.SendEphemeralPost(request.ChannelId, request.UserId, commandGenericError)
		w.WriteHeader(http.StatusOK)
		return
	}

	attachments := []*model.SlackAttachment(post.Attachments())
	p.API.LogDebug("Actions")
	host := ""
	service := ""
	for k := range attachments {
		actions := []*model.PostAction(attachments[k].Actions)
		for k := range actions {
			str := fmt.Sprintf("key[%s] value[%s]\n", k, actions[k].Integration.Context["host"])
			p.API.LogDebug("Host")
			p.API.LogDebug(str)
			str = fmt.Sprintf("key[%s] value[%s]\n", k, actions[k].Integration.Context["service"])
			p.API.LogDebug("Service")
			p.API.LogDebug(str)
			host = actions[k].Integration.Context["host"].(string)
			service = actions[k].Integration.Context["service"].(string)
		}
	}

	//Do post to check_mk here
	urlString := ""
	cmkHost := p.getConfiguration().CmkBaseUrl
	cmkUser := p.getConfiguration().CmkUsername
	cmkSecret := p.getConfiguration().CmkSecret

	if len(service) > 0 {
		urlString = fmt.Sprintf("%s/view.py?_username=%s&_secret=%s&_transid=-1&_do_confirm=yes&_do_actions=yes&_ack_comment=%s&_acknowledge=Acknowledge&host=%s&service=%s&view_name=service", cmkHost, cmkUser, cmkSecret, url.QueryEscape(p.ConvertCreatorIDToDisplayName(request.UserId)+" - "+theMessage), url.QueryEscape(host), url.QueryEscape(service))
	} else {
		urlString = fmt.Sprintf("%s/view.py?_username=%s&_secret=%s&_transid=-1&_do_confirm=yes&_do_actions=yes&_ack_comment=%s&_acknowledge=Acknowledge&host=%s&view_name=host", cmkHost, cmkUser, cmkSecret, url.QueryEscape(p.ConvertCreatorIDToDisplayName(request.UserId)+" - "+theMessage), url.QueryEscape(host))
	}

	p.API.LogDebug("Sending URL")
	p.API.LogDebug(urlString)

	resp, err := http.Get(urlString)
	if err != nil {
		p.API.LogDebug(err.Error())
	}
	p.API.LogDebug(fmt.Sprintf("Response Code: %d", resp.StatusCode))

	message := fmt.Sprintf("Acknowledged on host %s with message %s for service %s", host, theMessage, service)
	p.SendEphemeralPost(request.ChannelId, request.UserId, message)
	w.WriteHeader(http.StatusOK)
}

func writePostActionIntegrationResponse(w http.ResponseWriter, response *model.PostActionIntegrationResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response.ToJson())
}

// ConvertCreatorIDToDisplayName returns the display name to a given user ID of a poll creator
func (p *CheckMKPlugin) ConvertCreatorIDToDisplayName(creatorID string) string {
	user, err := p.API.GetUser(creatorID)
	if err != nil {
		return ""
	}
	displayName := user.GetDisplayName(model.SHOW_NICKNAME_FULLNAME)
	return displayName
}

func (p *CheckMKPlugin) SendEphemeralPost(channelID, userID, message string) {
	ephemeralPost := &model.Post{}
	ephemeralPost.ChannelId = channelID
	ephemeralPost.UserId = userID
	ephemeralPost.Message = message
	ephemeralPost.AddProp("override_username", "CMKBot")
	ephemeralPost.AddProp("from_webhook", "true")
	_ = p.API.SendEphemeralPost(userID, ephemeralPost)
}

// Main method starts plugin
func main() {
	plugin.ClientMain(&CheckMKPlugin{})
}
