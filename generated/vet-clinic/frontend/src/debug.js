// Debug WebSocket client for e2e testing
// Connects to /ws and handles eval requests from the server

let ws = null;
let sessionId = null;

export async function initDebug() {
  // Check if we have a session ID from a previous connection
  const savedSessionId = sessionStorage.getItem('debug_session_id');
  if (savedSessionId) {
    console.log('[debug] Reconnecting with saved session:', savedSessionId);
    connectWebSocket(savedSessionId);
  } else {
    // First connection - let server assign ID
    connectWebSocket(null);
  }
}

function connectWebSocket(requestedId) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    console.log('[debug] WebSocket already connected');
    return;
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  let wsUrl = `${protocol}//${window.location.host}/ws`;
  if (requestedId) {
    wsUrl += `?session=${requestedId}`;
  }

  ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('[debug] WebSocket connected');
  };

  ws.onmessage = async (event) => {
    try {
      const msg = JSON.parse(event.data);

      // Parse data field - might be string or object
      let data = msg.data;
      if (typeof data === 'string') {
        try {
          data = JSON.parse(data);
        } catch (e) {
          // Keep as string if not valid JSON
        }
      }

      if (msg.type === 'session') {
        // Session ID assigned - save for reconnection
        sessionId = data.session_id;
        sessionStorage.setItem('debug_session_id', sessionId);
        console.log('[debug] Session ID:', sessionId);
      } else if (msg.type === 'eval') {
        // Eval request from server
        const code = data.code;
        console.log('[debug] Eval request:', code);

        let result;
        let error = null;

        try {
          // Execute the code
          result = await eval(code);
        } catch (e) {
          error = e.message;
          console.error('[debug] Eval error:', e);
        }

        // Send response back - data should be an object, not a string
        const response = {
          id: msg.id,
          type: 'response',
          data: {
            result: result,
            error: error,
            type: typeof result
          }
        };

        ws.send(JSON.stringify(response));
        console.log('[debug] Sent response:', result);
      }
    } catch (e) {
      console.error('[debug] Message handling error:', e);
    }
  };

  ws.onclose = () => {
    console.log('[debug] WebSocket disconnected');
    // Reconnect after a delay with same session ID
    setTimeout(() => connectWebSocket(sessionId || requestedId), 2000);
  };

  ws.onerror = (error) => {
    console.error('[debug] WebSocket error:', error);
  };
}

export function getSessionId() {
  return sessionId;
}
