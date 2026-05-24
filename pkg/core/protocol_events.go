package core

import (
	"strings"
	"sync"

	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type protocolEventSubscriber struct {
	filter coreapi.EventFilter
	ch     chan protocol.Envelope
}

func (r *Runtime) subscribeProtocolEvents(filter coreapi.EventFilter, buffer int) (<-chan protocol.Envelope, func()) {
	if buffer <= 0 {
		buffer = 64
	}
	ch := make(chan protocol.Envelope, buffer)
	if r == nil {
		close(ch)
		return ch, func() {}
	}
	filter = normalizeProtocolEventFilter(filter)

	r.protocolEventsMu.Lock()
	if r.protocolEventSubscribers == nil {
		r.protocolEventSubscribers = map[int]protocolEventSubscriber{}
	}
	id := r.nextProtocolEventSubscriberID
	r.nextProtocolEventSubscriberID++
	r.protocolEventSubscribers[id] = protocolEventSubscriber{filter: filter, ch: ch}
	r.protocolEventsMu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			r.protocolEventsMu.Lock()
			if cur, ok := r.protocolEventSubscribers[id]; ok {
				delete(r.protocolEventSubscribers, id)
				close(cur.ch)
			}
			r.protocolEventsMu.Unlock()
		})
	}
	return ch, unsubscribe
}

func (r *Runtime) publishProtocolEvent(ev protocol.Envelope) {
	if r == nil {
		return
	}
	r.protocolEventsMu.RLock()
	defer r.protocolEventsMu.RUnlock()
	for _, sub := range r.protocolEventSubscribers {
		if !protocolEventMatchesFilter(ev, sub.filter) {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
		}
	}
}

func (r *Runtime) closeProtocolEventSubscribers() {
	if r == nil {
		return
	}
	r.protocolEventsMu.Lock()
	defer r.protocolEventsMu.Unlock()
	for id, sub := range r.protocolEventSubscribers {
		delete(r.protocolEventSubscribers, id)
		close(sub.ch)
	}
}

func normalizeProtocolEventFilter(filter coreapi.EventFilter) coreapi.EventFilter {
	filter.SessionID = strings.TrimSpace(filter.SessionID)
	filter.TurnID = strings.TrimSpace(filter.TurnID)
	filter.AgentID = strings.TrimSpace(filter.AgentID)
	return filter
}

func protocolEventMatchesFilter(ev protocol.Envelope, filter coreapi.EventFilter) bool {
	if filter.SessionID != "" && filter.SessionID != strings.TrimSpace(ev.SessionID) && filter.SessionID != protocolEventPayloadString(ev.Payload, "session_id") {
		return false
	}
	if filter.TurnID != "" &&
		filter.TurnID != strings.TrimSpace(ev.RequestID) &&
		filter.TurnID != strings.TrimSpace(ev.CorrelationID) &&
		filter.TurnID != protocolEventPayloadString(ev.Payload, "turn_id") &&
		filter.TurnID != protocolEventPayloadString(ev.Payload, "request_id") {
		return false
	}
	if filter.AgentID != "" &&
		filter.AgentID != protocolEventPayloadString(ev.Payload, "agent_id") &&
		filter.AgentID != protocolEventPayloadString(ev.Payload, "agent_name") {
		return false
	}
	return true
}

func protocolEventPayloadString(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}
