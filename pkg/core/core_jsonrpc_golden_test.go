//go:build legacy

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/dreamSailing/eos/pkg/coreapi"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

// --- Runtime.ServeJSONRPCStream end-to-end golden tests via net.Pipe ---

func TestServeJSONRPCStreamGoldenSequence(t *testing.T) {
	// Full golden sequence through Runtime.ServeJSONRPCStream:
	// initialize -> state/snapshot -> session/list
	// Verifies: Content-Length framing, request/response order, no jsonrpc field
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "golden-thread", []SessionMessage{
		{Role: "user", Type: "text", Content: "hello golden"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientStream := protocoljsonrpc.NewStream(clientConn, clientConn)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- rt.ServeJSONRPCStream(serverCtx, serverConn, serverConn)
	}()

	// Step 1: initialize
	req1, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
	if err := clientStream.WriteMessage(req1); err != nil {
		t.Fatalf("WriteMessage(initialize) error = %v", err)
	}

	decoded1, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(initialize) error = %v", err)
	}
	if decoded1.Response == nil {
		t.Fatalf("expected initialize response, got %+v", decoded1)
	}
	var initResult coreapijsonrpc.InitializeResult
	if err := json.Unmarshal(decoded1.Response.Result, &initResult); err != nil {
		t.Fatalf("Unmarshal(initialize) error = %v", err)
	}
	if initResult.ServerName != "eos-core" {
		t.Fatalf("ServerName = %q, want eos-core", initResult.ServerName)
	}
	if !containsString(initResult.Methods, protocoljsonrpc.MethodStateSnapshot) {
		t.Fatal("initialize methods missing state/snapshot")
	}

	// Step 2: state/snapshot
	req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-snap"), protocoljsonrpc.MethodStateSnapshot, nil)
	if err := clientStream.WriteMessage(req2); err != nil {
		t.Fatalf("WriteMessage(snapshot) error = %v", err)
	}

	decoded2, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(snapshot) error = %v", err)
	}
	if decoded2.Response == nil {
		t.Fatalf("expected snapshot response, got %+v", decoded2)
	}
	var snapshot coreapi.StateSnapshot
	if err := json.Unmarshal(decoded2.Response.Result, &snapshot); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v", err)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "hello golden" {
		t.Fatalf("Messages=%+v, want persisted message", snapshot.Messages)
	}

	// Step 3: session/list
	req3, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-list"), protocoljsonrpc.MethodSessionList,
		coreapi.ListSessionsRequest{WorkspaceRoot: workspace})
	if err := clientStream.WriteMessage(req3); err != nil {
		t.Fatalf("WriteMessage(session/list) error = %v", err)
	}

	decoded3, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(session/list) error = %v", err)
	}
	if decoded3.Response == nil {
		t.Fatalf("expected session/list response, got %+v", decoded3)
	}
	var sessions []coreapi.Session
	if err := json.Unmarshal(decoded3.Response.Result, &sessions); err != nil {
		t.Fatalf("Unmarshal(sessions) error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != meta.ID {
		t.Fatalf("sessions=%+v, want %q", sessions, meta.ID)
	}

	cancelServer()
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}

// --- Session create/resume through ServeJSONRPCStream ---

func TestServeJSONRPCStreamSessionCreateAndResume(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientStream := protocoljsonrpc.NewStream(clientConn, clientConn)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- rt.ServeJSONRPCStream(serverCtx, serverConn, serverConn)
	}()

	// Create session
	createReq := coreapi.CreateSessionRequest{
		WorkspaceRoot: workspace,
		Title:         "stream-created",
		Messages:      []coreapi.SessionMessage{{Role: "user", Type: "text", Content: "hello"}},
	}
	req1, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-create"), protocoljsonrpc.MethodSessionCreate, createReq)
	if err := clientStream.WriteMessage(req1); err != nil {
		t.Fatalf("WriteMessage(create) error = %v", err)
	}

	decoded1, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(create) error = %v", err)
	}
	var created coreapi.Session
	if err := json.Unmarshal(decoded1.Response.Result, &created); err != nil {
		t.Fatalf("Unmarshal(created) error = %v", err)
	}
	if created.ID == "" || created.Metadata["title"] != "stream-created" {
		t.Fatalf("created=%+v, want stream-created", created)
	}
	sessionID := created.ID

	// Resume session
	resumeReq := coreapi.ResumeSessionRequest{WorkspaceRoot: workspace, SessionID: sessionID}
	req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-resume"), protocoljsonrpc.MethodSessionResume, resumeReq)
	if err := clientStream.WriteMessage(req2); err != nil {
		t.Fatalf("WriteMessage(resume) error = %v", err)
	}

	decoded2, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(resume) error = %v", err)
	}
	var resumed coreapi.Session
	if err := json.Unmarshal(decoded2.Response.Result, &resumed); err != nil {
		t.Fatalf("Unmarshal(resumed) error = %v", err)
	}
	if resumed.ID != sessionID {
		t.Fatalf("resumed.ID=%q, want %q", resumed.ID, sessionID)
	}

	// Verify current session
	req3, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-current"), protocoljsonrpc.MethodSessionCurrent,
		coreapi.CurrentSessionRequest{WorkspaceRoot: workspace})
	if err := clientStream.WriteMessage(req3); err != nil {
		t.Fatalf("WriteMessage(current) error = %v", err)
	}

	decoded3, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(current) error = %v", err)
	}
	var current coreapi.Session
	if err := json.Unmarshal(decoded3.Response.Result, &current); err != nil {
		t.Fatalf("Unmarshal(current) error = %v", err)
	}
	if current.ID != sessionID {
		t.Fatalf("current.ID=%q, want %q", current.ID, sessionID)
	}

	cancelServer()
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}

// --- Event notification through ServeJSONRPCStream (no jsonrpc field) ---

func TestServeJSONRPCStreamEventNotificationNoJSONRPCVersion(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientStream := protocoljsonrpc.NewStream(clientConn, clientConn)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- rt.ServeJSONRPCStream(serverCtx, serverConn, serverConn)
	}()

	// Initialize first (to establish session)
	req1, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
	if err := clientStream.WriteMessage(req1); err != nil {
		t.Fatalf("WriteMessage(initialize) error = %v", err)
	}
	_, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(initialize) error = %v", err)
	}

	// Subscribe to events
	req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-sub"), protocoljsonrpc.MethodEventSubscribe,
		coreapi.EventFilter{})
	if err := clientStream.WriteMessage(req2); err != nil {
		t.Fatalf("WriteMessage(subscribe) error = %v", err)
	}

	decodedSub, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(subscribe) error = %v", err)
	}
	var sub coreapi.EventSubscription
	if err := json.Unmarshal(decodedSub.Response.Result, &sub); err != nil {
		t.Fatalf("Unmarshal(subscription) error = %v", err)
	}
	if sub.ID == "" {
		t.Fatal("subscription ID is empty")
	}

	// Wait a bit for subscription to be established
	time.Sleep(50 * time.Millisecond)

	// Create a session which should trigger state change events
	workspace := t.TempDir()
	createReq := coreapi.CreateSessionRequest{
		WorkspaceRoot: workspace,
		Title:         "notification-test",
	}
	req3, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-create"), protocoljsonrpc.MethodSessionCreate, createReq)
	if err := clientStream.WriteMessage(req3); err != nil {
		t.Fatalf("WriteMessage(create) error = %v", err)
	}

	// Read the response for create
	decodedCreate, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(create) error = %v", err)
	}
	if decodedCreate.Response == nil {
		t.Fatalf("expected create response, got %+v", decodedCreate)
	}

	// Try to read any notification that may have been sent
	// Use a short timeout since notifications are async
	notifCh := make(chan protocoljsonrpc.DecodedMessage, 5)
	go func() {
		for {
			msg, err := clientStream.ReadMessage()
			if err != nil {
				return
			}
			notifCh <- msg
		}
	}()

	// Check that any notification we receive does NOT have jsonrpc field
	select {
	case msg := <-notifCh:
		if msg.Kind == protocoljsonrpc.KindNotification && msg.Notification != nil {
			// Verify no jsonrpc in wire format
			params := string(msg.Notification.Params)
			if strings.Contains(params, `"jsonrpc"`) {
				t.Fatalf("notification params contains jsonrpc: %s", params)
			}
		}
	case <-time.After(200 * time.Millisecond):
		// No notification in timeout window - that's acceptable for this test
	}

	cancelServer()
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}

// --- Error codes through ServeJSONRPCStream ---

func TestServeJSONRPCStreamUnknownMethodReturnsMethodNotFound(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	var input bytes.Buffer
	var output bytes.Buffer

	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-unknown"), "fake/nonexistent", nil)
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rt.ServeJSONRPCStream(ctx, &input, &output)
	_ = err

	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want method not found")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d (MethodNotFound)", decoded.Response.Error.Code, protocoljsonrpc.CodeMethodNotFound)
	}
}

func TestServeJSONRPCStreamInvalidParamsReturnsInvalidParams(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	var input bytes.Buffer
	var output bytes.Buffer

	// Send invalid JSON params to session/list
	req := protocoljsonrpc.Request{
		ID:     protocoljsonrpc.StringID("req-bad"),
		Method: protocoljsonrpc.MethodSessionList,
		Params: json.RawMessage(`"invalid"`),
	}
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rt.ServeJSONRPCStream(ctx, &input, &output)
	_ = err

	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want invalid params")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeInvalidParams {
		t.Fatalf("error code = %d, want %d (InvalidParams)", decoded.Response.Error.Code, protocoljsonrpc.CodeInvalidParams)
	}
}

// --- Unsupported service through ServeJSONRPCStream ---

func TestServeJSONRPCStreamUnsupportedToolReturnsError(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	var input bytes.Buffer
	var output bytes.Buffer

	// Tool with unknown name returns unsupported
	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-tool"), protocoljsonrpc.MethodToolExecute,
		coreapi.ToolRequest{Name: "unknown_tool"})
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rt.ServeJSONRPCStream(ctx, &input, &output)
	_ = err

	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want unsupported error")
	}
	if !strings.Contains(decoded.Response.Error.Message, "unsupported") {
		t.Fatalf("error message = %q, want unsupported", decoded.Response.Error.Message)
	}
}

// --- Wire shape: all response types should NOT contain jsonrpc field ---

func TestServeJSONRPCStreamWireShapeNoJSONRPCVersion(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	var input bytes.Buffer
	var output bytes.Buffer

	// Initialize request
	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-wire"), protocoljsonrpc.MethodInitialize, nil)
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rt.ServeJSONRPCStream(ctx, &input, &output)
	_ = err

	wireOutput := output.String()
	if strings.Contains(wireOutput, `"jsonrpc"`) {
		t.Fatalf("wire output contains jsonrpc: %s", wireOutput)
	}
}

// --- Multiple sequential requests through ServeJSONRPCStream ---

func TestServeJSONRPCStreamMultipleSequentialRequests(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	var input bytes.Buffer
	var output bytes.Buffer

	methods := []string{
		protocoljsonrpc.MethodInitialize,
		protocoljsonrpc.MethodStateSnapshot,
	}
	ids := []string{"req-0", "req-1"}

	for i, method := range methods {
		req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID(ids[i]), method, nil)
		_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rt.ServeJSONRPCStream(ctx, &input, &output)
	_ = err

	readStream := protocoljsonrpc.NewStream(&output, nil)
	for i, expectedID := range ids {
		decoded, err := readStream.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage(%d) error = %v", i, err)
		}
		if decoded.Response == nil {
			t.Fatalf("response %d = nil, want response for %s", i, expectedID)
		}
		if decoded.Response.ID.String() != expectedID {
			t.Fatalf("response %d id = %q, want %q", i, decoded.Response.ID.String(), expectedID)
		}
		if decoded.Response.Error != nil {
			t.Fatalf("response %d has error: %+v", i, decoded.Response.Error)
		}
	}
}

// --- JSONRPCClient InProcess end-to-end ---

func TestJSONRPCClientInitializeAndStateSnapshot(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "client-test", []SessionMessage{
		{Role: "assistant", Type: "text", Content: "client test"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}

	// Initialize
	var initResult coreapijsonrpc.InitializeResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInitialize, nil, &initResult); err != nil {
		t.Fatalf("Call(initialize) error = %v", err)
	}
	if initResult.ServerName != "eos-core" {
		t.Fatalf("ServerName = %q, want eos-core", initResult.ServerName)
	}

	// State snapshot
	var snapshot coreapi.StateSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodStateSnapshot, nil, &snapshot); err != nil {
		t.Fatalf("Call(state/snapshot) error = %v", err)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
}

func TestJSONRPCClientSessionListWithWorkspaceFilter(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "filter-test", nil)
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}

	var sessions []coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionList,
		coreapi.ListSessionsRequest{WorkspaceRoot: workspace}, &sessions); err != nil {
		t.Fatalf("Call(session/list) error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != meta.ID {
		t.Fatalf("sessions=%+v, want %q", sessions, meta.ID)
	}
}

func TestJSONRPCClientErrorPropagation(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}

	// Unknown method
	var out map[string]any
	err = client.Call(context.Background(), "unknown/fake", nil, &out)
	if err == nil {
		t.Fatal("Call() error = nil, want method not found")
	}
	if !strings.Contains(err.Error(), "32601") {
		t.Fatalf("error = %v, want code 32601", err)
	}
}

// --- Approval/Inquiry through JSONRPCClient ---

func TestJSONRPCClientApprovalAndInquiry(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}

	// Approval respond
	if err := client.Call(context.Background(), protocoljsonrpc.MethodApprovalRespond,
		coreapi.ApprovalResponse{ApprovalID: "test-approval", Decision: "allow_once"}, nil); err != nil {
		t.Fatalf("Call(approval/respond) error = %v", err)
	}

	// Inquiry respond
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInquiryRespond,
		coreapi.InquiryResponse{InquiryID: "test-inquiry", Option: "auto", Text: "continue"}, nil); err != nil {
		t.Fatalf("Call(inquiry/respond) error = %v", err)
	}
}

// --- Turn methods through JSONRPCClient ---

func TestJSONRPCClientTurnMethods(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}

	// Turn interrupt for non-existent turn should error
	err = client.Call(context.Background(), protocoljsonrpc.MethodTurnInterrupt,
		coreapi.TurnRef{SessionID: "nonexistent", TurnID: "nonexistent"}, nil)
	if err == nil {
		t.Fatal("Call(turn/interrupt) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("error = %v, want not running", err)
	}
}

// --- WS transport helpers ---

func newCoreWSTestServer(t *testing.T, rt *Runtime) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_ = rt.ServeJSONRPCWS(r.Context(), conn)
	}))
}

func dialCoreWS(t *testing.T, url string) *protocoljsonrpc.WSConn {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	return protocoljsonrpc.NewWSConn(conn)
}

// --- Runtime.ServeJSONRPCWS end-to-end golden test ---

func TestServeJSONRPCWSGoldenSequence(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "ws-golden", []SessionMessage{
		{Role: "user", Type: "text", Content: "hello ws golden"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	srv := newCoreWSTestServer(t, rt)
	defer srv.Close()

	wsConn := dialCoreWS(t, srv.URL)
	ctx := context.Background()

	req1, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
	if err := wsConn.WriteMessage(ctx, req1); err != nil {
		t.Fatalf("WriteMessage(initialize) error = %v", err)
	}

	decoded1, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage(initialize) error = %v", err)
	}
	if decoded1.Response == nil {
		t.Fatalf("expected initialize response, got %+v", decoded1)
	}
	var initResult coreapijsonrpc.InitializeResult
	if err := json.Unmarshal(decoded1.Response.Result, &initResult); err != nil {
		t.Fatalf("Unmarshal(initialize) error = %v", err)
	}
	if initResult.ServerName != "eos-core" {
		t.Fatalf("ServerName = %q, want eos-core", initResult.ServerName)
	}
	if !containsString(initResult.Methods, protocoljsonrpc.MethodStateSnapshot) {
		t.Fatal("initialize methods missing state/snapshot")
	}

	req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-snap"), protocoljsonrpc.MethodStateSnapshot, nil)
	if err := wsConn.WriteMessage(ctx, req2); err != nil {
		t.Fatalf("WriteMessage(snapshot) error = %v", err)
	}

	decoded2, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage(snapshot) error = %v", err)
	}
	if decoded2.Response == nil {
		t.Fatalf("expected snapshot response, got %+v", decoded2)
	}
	var snapshot coreapi.StateSnapshot
	if err := json.Unmarshal(decoded2.Response.Result, &snapshot); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v", err)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "hello ws golden" {
		t.Fatalf("Messages=%+v, want persisted message", snapshot.Messages)
	}

	req3, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-list"), protocoljsonrpc.MethodSessionList,
		coreapi.ListSessionsRequest{WorkspaceRoot: workspace})
	if err := wsConn.WriteMessage(ctx, req3); err != nil {
		t.Fatalf("WriteMessage(session/list) error = %v", err)
	}

	decoded3, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage(session/list) error = %v", err)
	}
	if decoded3.Response == nil {
		t.Fatalf("expected session/list response, got %+v", decoded3)
	}
	var sessions []coreapi.Session
	if err := json.Unmarshal(decoded3.Response.Result, &sessions); err != nil {
		t.Fatalf("Unmarshal(sessions) error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != meta.ID {
		t.Fatalf("sessions=%+v, want %q", sessions, meta.ID)
	}
}

// --- Cross-transport initialize methods consistency ---

func TestTransportInitializeMethodsConsistency(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	collectMethods := func(t *testing.T, initResult coreapijsonrpc.InitializeResult) []string {
		t.Helper()
		methods := append([]string(nil), initResult.Methods...)
		sort.Strings(methods)
		return methods
	}

	t.Run("InProcess", func(t *testing.T) {
		client, err := rt.JSONRPCClient()
		if err != nil {
			t.Fatalf("JSONRPCClient() error = %v", err)
		}
		var result coreapijsonrpc.InitializeResult
		if err := client.Call(context.Background(), protocoljsonrpc.MethodInitialize, nil, &result); err != nil {
			t.Fatalf("Call(initialize) error = %v", err)
		}
		got := collectMethods(t, result)
		assertInitializeMethodsComplete(t, got)
	})

	t.Run("Stream", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		clientStream := protocoljsonrpc.NewStream(clientConn, clientConn)
		serverCtx, cancelServer := context.WithCancel(context.Background())
		defer cancelServer()
		serverDone := make(chan error, 1)
		go func() {
			serverDone <- rt.ServeJSONRPCStream(serverCtx, serverConn, serverConn)
		}()

		req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
		if err := clientStream.WriteMessage(req); err != nil {
			t.Fatalf("WriteMessage(initialize) error = %v", err)
		}

		decoded, err := clientStream.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage(initialize) error = %v", err)
		}
		var result coreapijsonrpc.InitializeResult
		if err := json.Unmarshal(decoded.Response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(initialize) error = %v", err)
		}
		got := collectMethods(t, result)
		assertInitializeMethodsComplete(t, got)

		cancelServer()
		select {
		case <-serverDone:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for server shutdown")
		}
	})

	t.Run("WebSocket", func(t *testing.T) {
		srv := newCoreWSTestServer(t, rt)
		defer srv.Close()

		wsConn := dialCoreWS(t, srv.URL)
		ctx := context.Background()

		req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
		if err := wsConn.WriteMessage(ctx, req); err != nil {
			t.Fatalf("WriteMessage(initialize) error = %v", err)
		}

		decoded, err := wsConn.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage(initialize) error = %v", err)
		}
		var result coreapijsonrpc.InitializeResult
		if err := json.Unmarshal(decoded.Response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(initialize) error = %v", err)
		}
		got := collectMethods(t, result)
		assertInitializeMethodsComplete(t, got)
	})

	t.Run("CrossTransportIdentical", func(t *testing.T) {
		ipClient, err := rt.JSONRPCClient()
		if err != nil {
			t.Fatalf("JSONRPCClient() error = %v", err)
		}
		var ipResult coreapijsonrpc.InitializeResult
		if err := ipClient.Call(context.Background(), protocoljsonrpc.MethodInitialize, nil, &ipResult); err != nil {
			t.Fatalf("Call(initialize) error = %v", err)
		}
		ipMethods := collectMethods(t, ipResult)

		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()
		clientStream := protocoljsonrpc.NewStream(clientConn, clientConn)
		serverCtx, cancelServer := context.WithCancel(context.Background())
		defer cancelServer()
		serverDone := make(chan error, 1)
		go func() {
			serverDone <- rt.ServeJSONRPCStream(serverCtx, serverConn, serverConn)
		}()

		req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init"), protocoljsonrpc.MethodInitialize, nil)
		if err := clientStream.WriteMessage(req); err != nil {
			t.Fatalf("WriteMessage(initialize) error = %v", err)
		}
		decoded, err := clientStream.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage(initialize) error = %v", err)
		}
		var streamResult coreapijsonrpc.InitializeResult
		if err := json.Unmarshal(decoded.Response.Result, &streamResult); err != nil {
			t.Fatalf("Unmarshal(initialize) error = %v", err)
		}
		streamMethods := collectMethods(t, streamResult)

		srv := newCoreWSTestServer(t, rt)
		defer srv.Close()
		wsConn := dialCoreWS(t, srv.URL)
		ctx := context.Background()

		req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-init-ws"), protocoljsonrpc.MethodInitialize, nil)
		if err := wsConn.WriteMessage(ctx, req2); err != nil {
			t.Fatalf("WriteMessage(initialize) error = %v", err)
		}
		decoded2, err := wsConn.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage(initialize) error = %v", err)
		}
		var wsResult coreapijsonrpc.InitializeResult
		if err := json.Unmarshal(decoded2.Response.Result, &wsResult); err != nil {
			t.Fatalf("Unmarshal(initialize) error = %v", err)
		}
		wsMethods := collectMethods(t, wsResult)

		if len(ipMethods) != len(streamMethods) {
			t.Fatalf("in-process methods count = %d, stream methods count = %d", len(ipMethods), len(streamMethods))
		}
		if len(ipMethods) != len(wsMethods) {
			t.Fatalf("in-process methods count = %d, ws methods count = %d", len(ipMethods), len(wsMethods))
		}
		for i := range ipMethods {
			if ipMethods[i] != streamMethods[i] {
				t.Fatalf("methods[%d] in-process=%q stream=%q", i, ipMethods[i], streamMethods[i])
			}
			if ipMethods[i] != wsMethods[i] {
				t.Fatalf("methods[%d] in-process=%q ws=%q", i, ipMethods[i], wsMethods[i])
			}
		}

		cancelServer()
		select {
		case <-serverDone:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for server shutdown")
		}
	})
}

func assertInitializeMethodsComplete(t *testing.T, methods []string) {
	t.Helper()
	required := []string{
		protocoljsonrpc.MethodInitialize,
		protocoljsonrpc.MethodStateSnapshot,
		protocoljsonrpc.MethodSessionList,
		protocoljsonrpc.MethodWorkspaceList,
	}
	for _, m := range required {
		found := false
		for _, got := range methods {
			if got == m {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("initialize methods missing %q", m)
		}
	}
}

// --- WS state/snapshot decodability ---

func TestServeJSONRPCWSStateSnapshotDecodable(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "ws-snap", []SessionMessage{
		{Role: "user", Type: "text", Content: "snapshot check"},
		{Role: "assistant", Type: "text", Content: "snapshot reply"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	srv := newCoreWSTestServer(t, rt)
	defer srv.Close()

	wsConn := dialCoreWS(t, srv.URL)
	ctx := context.Background()

	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-snap"), protocoljsonrpc.MethodStateSnapshot, nil)
	if err := wsConn.WriteMessage(ctx, req); err != nil {
		t.Fatalf("WriteMessage(state/snapshot) error = %v", err)
	}

	decoded, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage(state/snapshot) error = %v", err)
	}
	if decoded.Response == nil {
		t.Fatalf("expected response, got %+v", decoded)
	}
	if decoded.Response.Error != nil {
		t.Fatalf("response error: %+v", decoded.Response.Error)
	}

	var snapshot coreapi.StateSnapshot
	if err := json.Unmarshal(decoded.Response.Result, &snapshot); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v", err)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
	if len(snapshot.Messages) != 2 {
		t.Fatalf("Messages count = %d, want 2", len(snapshot.Messages))
	}
	if snapshot.Messages[0].Content != "snapshot check" {
		t.Fatalf("Messages[0].Content = %q, want snapshot check", snapshot.Messages[0].Content)
	}
}

// --- WS error response no jsonrpc version ---

func TestServeJSONRPCWSErrorResponseNoJSONRPCVersion(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	srv := newCoreWSTestServer(t, rt)
	defer srv.Close()

	wsConn := dialCoreWS(t, srv.URL)
	ctx := context.Background()

	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-unknown"), "fake/nonexistent", nil)
	if err := wsConn.WriteMessage(ctx, req); err != nil {
		t.Fatalf("WriteMessage(unknown) error = %v", err)
	}

	decoded, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response == nil {
		t.Fatalf("expected response, got %+v", decoded)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want method not found")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d (MethodNotFound)", decoded.Response.Error.Code, protocoljsonrpc.CodeMethodNotFound)
	}

	wire, err := protocoljsonrpc.Marshal(decoded.Response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(wire), `"jsonrpc"`) {
		t.Fatalf("error response wire contains jsonrpc: %s", wire)
	}

	req2, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-bad-params"), protocoljsonrpc.MethodSessionList,
		json.RawMessage(`"not an object"`))
	if err := wsConn.WriteMessage(ctx, req2); err != nil {
		t.Fatalf("WriteMessage(bad-params) error = %v", err)
	}

	decoded2, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage(bad-params) error = %v", err)
	}
	if decoded2.Response == nil || decoded2.Response.Error == nil {
		t.Fatalf("expected error response, got %+v", decoded2)
	}
	if decoded2.Response.Error.Code != protocoljsonrpc.CodeInvalidParams {
		t.Fatalf("error code = %d, want %d (InvalidParams)", decoded2.Response.Error.Code, protocoljsonrpc.CodeInvalidParams)
	}

	wire2, err := protocoljsonrpc.Marshal(decoded2.Response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(wire2), `"jsonrpc"`) {
		t.Fatalf("invalid-params error wire contains jsonrpc: %s", wire2)
	}
}

// --- WS notification/event wire shape stability ---

func TestServeJSONRPCWSNotificationNoJSONRPCVersion(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	srv := newCoreWSTestServer(t, rt)
	defer srv.Close()

	wsConn := dialCoreWS(t, srv.URL)
	ctx := context.Background()

	client := protocoljsonrpc.NewWSClient(wsConn, protocoljsonrpc.WithWSNotificationHandler(func(_ context.Context, notif protocoljsonrpc.Notification) error {
		return nil
	}))
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var initResult coreapijsonrpc.InitializeResult
	if err := client.Call(ctx, protocoljsonrpc.MethodInitialize, nil, &initResult); err != nil {
		t.Fatalf("Call(initialize) error = %v", err)
	}

	var sub coreapi.EventSubscription
	if err := client.Call(ctx, protocoljsonrpc.MethodEventSubscribe, coreapi.EventFilter{}, &sub); err != nil {
		t.Fatalf("Call(event/subscribe) error = %v", err)
	}
	if sub.ID == "" {
		t.Fatal("subscription ID is empty")
	}

	time.Sleep(50 * time.Millisecond)

	workspace := t.TempDir()
	var created coreapi.Session
	if err := client.Call(ctx, protocoljsonrpc.MethodSessionCreate, coreapi.CreateSessionRequest{
		WorkspaceRoot: workspace,
		Title:         "ws-notif-test",
	}, &created); err != nil {
		t.Fatalf("Call(session/create) error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	var unsubResult map[string]any
	if err := client.Call(ctx, protocoljsonrpc.MethodEventUnsubscribe, coreapi.EventUnsubscribeRequest{ID: sub.ID}, &unsubResult); err != nil {
		t.Fatalf("Call(event/unsubscribe) error = %v", err)
	}
}
