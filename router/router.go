package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/CAFxX/httpcompression"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/owncast/owncast/activitypub"
	"github.com/owncast/owncast/config"
	"github.com/owncast/owncast/controllers"
	"github.com/owncast/owncast/controllers/admin"
	fediverseauth "github.com/owncast/owncast/controllers/auth/fediverse"
	"github.com/owncast/owncast/controllers/auth/indieauth"
	"github.com/owncast/owncast/controllers/moderation"
	"github.com/owncast/owncast/core/chat"
	"github.com/owncast/owncast/core/data"
	"github.com/owncast/owncast/core/user"
	"github.com/owncast/owncast/router/middleware"
	"github.com/owncast/owncast/utils"
	"github.com/owncast/owncast/yp"
)

// Start starts the router for the http, ws, and rtmp.
func Start() error {
	// The primary web app.
	http.HandleFunc("/", controllers.IndexHandler)

	// The admin web app.
	http.HandleFunc("/admin/", middleware.RequireAdminAuth(controllers.IndexHandler))

	// Images
	http.HandleFunc("/thumbnail.jpg", controllers.GetThumbnail)
	http.HandleFunc("/preview.gif", controllers.GetPreview)
	http.HandleFunc("/logo", controllers.GetLogo)

	// Custom Javascript
	http.HandleFunc("/customjavascript", controllers.ServeCustomJavascript)

	// Return a single emoji image.
	http.HandleFunc(config.EmojiDir, controllers.GetCustomEmojiImage)

	// return the logo

	// return a logo that's compatible with external social networks
	http.HandleFunc("/logo/external", controllers.GetCompatibleLogo)

	// robots.txt
	http.HandleFunc("/robots.txt", controllers.GetRobotsDotTxt)

	// status of the system
	http.HandleFunc("/api/status", controllers.GetStatus)

	// custom emoji supported in the chat
	http.HandleFunc("/api/emoji", controllers.GetCustomEmojiList)

	// chat rest api
	http.HandleFunc("/api/chat", middleware.RequireUserAccessToken(controllers.GetChatMessages))

	// web config api
	http.HandleFunc("/api/config", controllers.GetWebConfig)

	// return the YP protocol data
	http.HandleFunc("/api/yp", yp.GetYPResponse)

	// list of all social platforms
	http.HandleFunc("/api/socialplatforms", controllers.GetAllSocialPlatforms)

	// return the list of video variants available
	http.HandleFunc("/api/video/variants", controllers.GetVideoStreamOutputVariants)

	// tell the backend you're an active viewer
	http.HandleFunc("/api/ping", controllers.Ping)

	// register a new chat user
	http.HandleFunc("/api/chat/register", controllers.RegisterAnonymousChatUser)

	// return remote follow details
	http.HandleFunc("/api/remotefollow", controllers.RemoteFollow)

	// return followers
	http.HandleFunc("/api/followers", middleware.HandlePagination(controllers.GetFollowers))

	// save client video playback metrics
	http.HandleFunc("/api/metrics/playback", controllers.ReportPlaybackMetrics)

	// Register for notifications
	http.HandleFunc("/api/notifications/register", middleware.RequireUserAccessToken(controllers.RegisterForLiveNotifications))

	// Authenticated admin requests

	// Current inbound broadcaster
	http.HandleFunc("/api/admin/status", middleware.RequireAdminAuth(admin.Status))

	// Return HLS video
	http.HandleFunc("/hls/", controllers.HandleHLSRequest)

	// Disconnect inbound stream
	http.HandleFunc("/api/admin/disconnect", middleware.RequireAdminAuth(admin.DisconnectInboundConnection))

	// Server config
	http.HandleFunc("/api/admin/serverconfig", middleware.RequireAdminAuth(admin.GetServerConfig))

	// Get viewer count over time
	http.HandleFunc("/api/admin/viewersOverTime", middleware.RequireAdminAuth(admin.GetViewersOverTime))

	// Get active viewers
	http.HandleFunc("/api/admin/viewers", middleware.RequireAdminAuth(admin.GetActiveViewers))

	// Get hardware stats
	http.HandleFunc("/api/admin/hardwarestats", middleware.RequireAdminAuth(admin.GetHardwareStats))

	// Get a a detailed list of currently connected chat clients
	http.HandleFunc("/api/admin/chat/clients", middleware.RequireAdminAuth(admin.GetConnectedChatClients))

	// Get all logs
	http.HandleFunc("/api/admin/logs", middleware.RequireAdminAuth(admin.GetLogs))

	// Get warning/error logs
	http.HandleFunc("/api/admin/logs/warnings", middleware.RequireAdminAuth(admin.GetWarnings))

	// Get all chat messages for the admin, unfiltered.
	http.HandleFunc("/api/admin/chat/messages", middleware.RequireAdminAuth(admin.GetChatMessages))

	// Update chat message visibility
	http.HandleFunc("/api/admin/chat/messagevisibility", middleware.RequireAdminAuth(admin.UpdateMessageVisibility))

	// Enable/disable a user
	http.HandleFunc("/api/admin/chat/users/setenabled", middleware.RequireAdminAuth(admin.UpdateUserEnabled))

	// Ban/unban an IP address
	http.HandleFunc("/api/admin/chat/users/ipbans/create", middleware.RequireAdminAuth(admin.BanIPAddress))

	// Remove an IP address ban
	http.HandleFunc("/api/admin/chat/users/ipbans/remove", middleware.RequireAdminAuth(admin.UnBanIPAddress))

	// Return all the banned IP addresses
	http.HandleFunc("/api/admin/chat/users/ipbans", middleware.RequireAdminAuth(admin.GetIPAddressBans))

	// Get a list of disabled users
	http.HandleFunc("/api/admin/chat/users/disabled", middleware.RequireAdminAuth(admin.GetDisabledUsers))

	// Set moderator status for a user
	http.HandleFunc("/api/admin/chat/users/setmoderator", middleware.RequireAdminAuth(admin.UpdateUserModerator))

	// Get a list of moderator users
	http.HandleFunc("/api/admin/chat/users/moderators", middleware.RequireAdminAuth(admin.GetModerators))

	// return followers
	http.HandleFunc("/api/admin/followers", middleware.RequireAdminAuth(middleware.HandlePagination(controllers.GetFollowers)))

	// Get a list of pending follow requests
	http.HandleFunc("/api/admin/followers/pending", middleware.RequireAdminAuth(admin.GetPendingFollowRequests))

	// Get a list of rejected or blocked follows
	http.HandleFunc("/api/admin/followers/blocked", middleware.RequireAdminAuth(admin.GetBlockedAndRejectedFollowers))

	// Set the following state of a follower or follow request.
	http.HandleFunc("/api/admin/followers/approve", middleware.RequireAdminAuth(admin.ApproveFollower))

	// Upload custom emoji
	http.HandleFunc("/api/admin/emoji/upload", middleware.RequireAdminAuth(admin.UploadCustomEmoji))

	// Delete custom emoji
	http.HandleFunc("/api/admin/emoji/delete", middleware.RequireAdminAuth(admin.DeleteCustomEmoji))

	// Update config values

	// Change the current streaming key in memory
	http.HandleFunc("/api/admin/config/adminpass", middleware.RequireAdminAuth(admin.SetAdminPassword))

	//  Set an array of valid stream keys
	http.HandleFunc("/api/admin/config/streamkeys", middleware.RequireAdminAuth(admin.SetStreamKeys))

	// Change the extra page content in memory
	http.HandleFunc("/api/admin/config/pagecontent", middleware.RequireAdminAuth(admin.SetExtraPageContent))

	// Stream title
	http.HandleFunc("/api/admin/config/streamtitle", middleware.RequireAdminAuth(admin.SetStreamTitle))

	// Server name
	http.HandleFunc("/api/admin/config/name", middleware.RequireAdminAuth(admin.SetServerName))

	// Server summary
	http.HandleFunc("/api/admin/config/serversummary", middleware.RequireAdminAuth(admin.SetServerSummary))

	// Offline message
	http.HandleFunc("/api/admin/config/offlinemessage", middleware.RequireAdminAuth(admin.SetCustomOfflineMessage))

	// Server welcome message
	http.HandleFunc("/api/admin/config/welcomemessage", middleware.RequireAdminAuth(admin.SetServerWelcomeMessage))

	// Disable chat
	http.HandleFunc("/api/admin/config/chat/disable", middleware.RequireAdminAuth(admin.SetChatDisabled))

	// Disable chat user join messages
	http.HandleFunc("/api/admin/config/chat/joinmessagesenabled", middleware.RequireAdminAuth(admin.SetChatJoinMessagesEnabled))

	// Enable/disable chat established user mode
	http.HandleFunc("/api/admin/config/chat/establishedusermode", middleware.RequireAdminAuth(admin.SetEnableEstablishedChatUserMode))

	// Set chat usernames that are not allowed
	http.HandleFunc("/api/admin/config/chat/forbiddenusernames", middleware.RequireAdminAuth(admin.SetForbiddenUsernameList))

	// Set the suggested chat usernames that will be assigned automatically
	http.HandleFunc("/api/admin/config/chat/suggestedusernames", middleware.RequireAdminAuth(admin.SetSuggestedUsernameList))

	// Set video codec
	http.HandleFunc("/api/admin/config/video/codec", middleware.RequireAdminAuth(admin.SetVideoCodec))

	// Set style/color/css values
	http.HandleFunc("/api/admin/config/appearance", middleware.RequireAdminAuth(admin.SetCustomColorVariableValues))

	// Return all webhooks
	http.HandleFunc("/api/admin/webhooks", middleware.RequireAdminAuth(admin.GetWebhooks))

	// Delete a single webhook
	http.HandleFunc("/api/admin/webhooks/delete", middleware.RequireAdminAuth(admin.DeleteWebhook))

	// Create a single webhook
	http.HandleFunc("/api/admin/webhooks/create", middleware.RequireAdminAuth(admin.CreateWebhook))

	// Get all access tokens
	http.HandleFunc("/api/admin/accesstokens", middleware.RequireAdminAuth(admin.GetExternalAPIUsers))

	// Delete a single access token
	http.HandleFunc("/api/admin/accesstokens/delete", middleware.RequireAdminAuth(admin.DeleteExternalAPIUser))

	// Create a single access token
	http.HandleFunc("/api/admin/accesstokens/create", middleware.RequireAdminAuth(admin.CreateExternalAPIUser))

	// Return the auto-update features that are supported for this instance.
	http.HandleFunc("/api/admin/update/options", middleware.RequireAdminAuth(admin.AutoUpdateOptions))

	// Begin the auto update
	http.HandleFunc("/api/admin/update/start", middleware.RequireAdminAuth(admin.AutoUpdateStart))

	// Force quit the service to restart it
	http.HandleFunc("/api/admin/update/forcequit", middleware.RequireAdminAuth(admin.AutoUpdateForceQuit))

	// Send a system message to chat
	http.HandleFunc("/api/integrations/chat/system", middleware.RequireExternalAPIAccessToken(user.ScopeCanSendSystemMessages, admin.SendSystemMessage))

	// Send a system message to a single client
	http.HandleFunc(utils.RestEndpoint("/api/integrations/chat/system/client/{clientId}", middleware.RequireExternalAPIAccessToken(user.ScopeCanSendSystemMessages, admin.SendSystemMessageToConnectedClient)))

	// Send a user message to chat *NO LONGER SUPPORTED
	http.HandleFunc("/api/integrations/chat/user", middleware.RequireExternalAPIAccessToken(user.ScopeCanSendChatMessages, admin.SendUserMessage))

	// Send a message to chat as a specific 3rd party bot/integration based on its access token
	http.HandleFunc("/api/integrations/chat/send", middleware.RequireExternalAPIAccessToken(user.ScopeCanSendChatMessages, admin.SendIntegrationChatMessage))

	// Send a user action to chat
	http.HandleFunc("/api/integrations/chat/action", middleware.RequireExternalAPIAccessToken(user.ScopeCanSendSystemMessages, admin.SendChatAction))

	// Hide chat message
	http.HandleFunc("/api/integrations/chat/messagevisibility", middleware.RequireExternalAPIAccessToken(user.ScopeHasAdminAccess, admin.ExternalUpdateMessageVisibility))

	// Stream title
	http.HandleFunc("/api/integrations/streamtitle", middleware.RequireExternalAPIAccessToken(user.ScopeHasAdminAccess, admin.ExternalSetStreamTitle))

	// Get chat history
	http.HandleFunc("/api/integrations/chat", middleware.RequireExternalAPIAccessToken(user.ScopeHasAdminAccess, controllers.ExternalGetChatMessages))

	// Connected clients
	http.HandleFunc("/api/integrations/clients", middleware.RequireExternalAPIAccessToken(user.ScopeHasAdminAccess, admin.ExternalGetConnectedChatClients))

	// Logo path
	http.HandleFunc("/api/admin/config/logo", middleware.RequireAdminAuth(admin.SetLogo))

	// Server tags
	http.HandleFunc("/api/admin/config/tags", middleware.RequireAdminAuth(admin.SetTags))

	// ffmpeg
	http.HandleFunc("/api/admin/config/ffmpegpath", middleware.RequireAdminAuth(admin.SetFfmpegPath))

	// Server http port
	http.HandleFunc("/api/admin/config/webserverport", middleware.RequireAdminAuth(admin.SetWebServerPort))

	// Server http listen address
	http.HandleFunc("/api/admin/config/webserverip", middleware.RequireAdminAuth(admin.SetWebServerIP))

	// Server rtmp port
	http.HandleFunc("/api/admin/config/rtmpserverport", middleware.RequireAdminAuth(admin.SetRTMPServerPort))

	// Server webrtc port
	http.HandleFunc("/api/admin/config/webrtcserverport", middleware.RequireAdminAuth(admin.SetWebRTCServerPort))
	
	// Server webrtc udp mux port
	http.HandleFunc("/api/admin/config/webrtcudpmuxport", middleware.RequireAdminAuth(admin.SetWebRTCUDPMuxPort))

	// Server stream mode
	http.HandleFunc("/api/admin/config/streammode", middleware.RequireAdminAuth(admin.SetStreamMode))

	// Websocket host override
	http.HandleFunc("/api/admin/config/sockethostoverride", middleware.RequireAdminAuth(admin.SetSocketHostOverride))

	// Custom video serving endpoint
	http.HandleFunc("/api/admin/config/videoservingendpoint", middleware.RequireAdminAuth(admin.SetVideoServingEndpoint))

	// Is server marked as NSFW
	http.HandleFunc("/api/admin/config/nsfw", middleware.RequireAdminAuth(admin.SetNSFW))

	// directory enabled
	http.HandleFunc("/api/admin/config/directoryenabled", middleware.RequireAdminAuth(admin.SetDirectoryEnabled))

	// social handles
	http.HandleFunc("/api/admin/config/socialhandles", middleware.RequireAdminAuth(admin.SetSocialHandles))

	// set the number of video segments and duration per segment in a playlist
	http.HandleFunc("/api/admin/config/video/streamlatencylevel", middleware.RequireAdminAuth(admin.SetStreamLatencyLevel))

	// set an array of video output configurations
	http.HandleFunc("/api/admin/config/video/streamoutputvariants", middleware.RequireAdminAuth(admin.SetStreamOutputVariants))

	// set s3 configuration
	http.HandleFunc("/api/admin/config/s3", middleware.RequireAdminAuth(admin.SetS3Configuration))

	// set server url
	http.HandleFunc("/api/admin/config/serverurl", middleware.RequireAdminAuth(admin.SetServerURL))

	// reset the YP registration
	http.HandleFunc("/api/admin/yp/reset", middleware.RequireAdminAuth(admin.ResetYPRegistration))

	// set external action links
	http.HandleFunc("/api/admin/config/externalactions", middleware.RequireAdminAuth(admin.SetExternalActions))

	// set custom style css
	http.HandleFunc("/api/admin/config/customstyles", middleware.RequireAdminAuth(admin.SetCustomStyles))

	// set custom style javascript
	http.HandleFunc("/api/admin/config/customjavascript", middleware.RequireAdminAuth(admin.SetCustomJavascript))

	// Video playback metrics
	http.HandleFunc("/api/admin/metrics/video", middleware.RequireAdminAuth(admin.GetVideoPlaybackMetrics))

	// Is the viewer count hidden from viewers
	http.HandleFunc("/api/admin/config/hideviewercount", middleware.RequireAdminAuth(admin.SetHideViewerCount))

	// set disabling of search indexing
	http.HandleFunc("/api/admin/config/disablesearchindexing", middleware.RequireAdminAuth(admin.SetDisableSearchIndexing))

	// Inline chat moderation actions

	// Update chat message visibility
	http.HandleFunc("/api/chat/messagevisibility", middleware.RequireUserModerationScopeAccesstoken(admin.UpdateMessageVisibility))

	// Enable/disable a user
	http.HandleFunc("/api/chat/users/setenabled", middleware.RequireUserModerationScopeAccesstoken(admin.UpdateUserEnabled))

	// Get a user's details
	http.HandleFunc("/api/moderation/chat/user/", middleware.RequireUserModerationScopeAccesstoken(moderation.GetUserDetails))

	// Configure Federation features

	// enable/disable federation features
	http.HandleFunc("/api/admin/config/federation/enable", middleware.RequireAdminAuth(admin.SetFederationEnabled))

	// set if federation activities are private
	http.HandleFunc("/api/admin/config/federation/private", middleware.RequireAdminAuth(admin.SetFederationActivityPrivate))

	// set if fediverse engagement appears in chat
	http.HandleFunc("/api/admin/config/federation/showengagement", middleware.RequireAdminAuth(admin.SetFederationShowEngagement))

	// set local federated username
	http.HandleFunc("/api/admin/config/federation/username", middleware.RequireAdminAuth(admin.SetFederationUsername))

	// set federated go live message
	http.HandleFunc("/api/admin/config/federation/livemessage", middleware.RequireAdminAuth(admin.SetFederationGoLiveMessage))

	// Federation blocked domains
	http.HandleFunc("/api/admin/config/federation/blockdomains", middleware.RequireAdminAuth(admin.SetFederationBlockDomains))

	// send a public message to the Fediverse from the server's user
	http.HandleFunc("/api/admin/federation/send", middleware.RequireAdminAuth(admin.SendFederatedMessage))

	// Return federated activities
	http.HandleFunc("/api/admin/federation/actions", middleware.RequireAdminAuth(middleware.HandlePagination(admin.GetFederatedActions)))

	// Prometheus metrics
	http.Handle("/api/admin/prometheus", middleware.RequireAdminAuth(func(rw http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(rw, r)
	}))

	// Configure outbound notification channels.
	http.HandleFunc("/api/admin/config/notifications/discord", middleware.RequireAdminAuth(admin.SetDiscordNotificationConfiguration))
	http.HandleFunc("/api/admin/config/notifications/browser", middleware.RequireAdminAuth(admin.SetBrowserNotificationConfiguration))

	// Auth

	// Start auth flow
	http.HandleFunc("/api/auth/indieauth", middleware.RequireUserAccessToken(indieauth.StartAuthFlow))
	http.HandleFunc("/api/auth/indieauth/callback", indieauth.HandleRedirect)
	http.HandleFunc("/api/auth/provider/indieauth", indieauth.HandleAuthEndpoint)

	http.HandleFunc("/api/auth/fediverse", middleware.RequireUserAccessToken(fediverseauth.RegisterFediverseOTPRequest))
	http.HandleFunc("/api/auth/fediverse/verify", fediverseauth.VerifyFediverseOTPRequest)

	// ActivityPub has its own router
	activitypub.Start(data.GetDatastore())

	// websocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		chat.HandleClientConnection(w, r)
	})

	// Optional public static files
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir(config.PublicFilesPath))))

	port := config.WebServerPort
	ip := config.WebServerIP

	h2s := &http2.Server{}

	// Create a custom mux handler to intercept the /debug/vars endpoint.
	// This is a hack because Prometheus enables this endpoint by default
	// due to its use of expvar and we do not want this exposed.
	defaultMux := h2c.NewHandler(http.DefaultServeMux, h2s)
	m := http.NewServeMux()

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/debug/vars" {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if r.URL.Path == "/embed/chat/" || r.URL.Path == "/embed/chat" {
			// Redirect /embed/chat
			http.Redirect(w, r, "/embed/chat/readonly", http.StatusTemporaryRedirect)
		} else {
			defaultMux.ServeHTTP(w, r)
		}
	})

	compress, _ := httpcompression.DefaultAdapter() // Use the default configuration
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", ip, port),
		ReadHeaderTimeout: 4 * time.Second,
		Handler:           compress(m),
	}

	if ip != "0.0.0.0" {
		log.Infof("Web server is listening at %s:%d.", ip, port)
	} else {
		log.Infof("Web server is listening on port %d.", port)
	}
	log.Infoln("Configure this server by visiting /admin.")

	return server.ListenAndServe()
}
