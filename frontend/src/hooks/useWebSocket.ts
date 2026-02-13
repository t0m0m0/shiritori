import { useRef, useCallback, useEffect } from 'react';
import type { OutgoingMessage, IncomingMessage } from '../types/messages';

type MessageHandler = (msg: IncomingMessage) => void;

export function useWebSocket(onMessage: MessageHandler) {
  const wsRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState <= 1) return;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WS connected');
    };

    ws.onclose = () => {
      console.log('WS closed');
      reconnectTimerRef.current = setTimeout(connect, 2000);
    };

    ws.onerror = (e) => console.error('WS error', e);

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data) as IncomingMessage;
        onMessageRef.current(msg);
      } catch (err) {
        console.error('Failed to parse WS message', err);
      }
    };
  }, []);

  const send = useCallback((msg: OutgoingMessage) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { send, wsRef };
}
