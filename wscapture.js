let glContext = null;
const maxOutstanding = 10;

function isGLContext(gl) {
  return (
    gl instanceof WebGLRenderingContext || gl instanceof WebGL2RenderingContext
  );
}

export async function setContext(gl) {
  if (gl == null) {
    glContext = null;
    return;
  }
  if (!isGLContext(gl)) {
    console.error('Not a WebGLRenderingContext');
    return;
  }
  glContext = gl;
}

// Steal the result of the next call to getContext that returns a WebGL context.
export function stealContext() {
  const oldGetContext = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function getContext() {
    const result = oldGetContext.apply(this, arguments);
    if (isGLContext(result)) {
      glContext = result;
    } else {
      console.warn('Not a WebGLRenderingContext', result);
    }
    HTMLCanvasElement.prototype.getContext = oldGetContext;
    return result;
  };
}

let ws = null;
let recording = null;
let closing = false;

function reset() {
  if (ws != null) {
    ws.close();
    recording = null;
    closing = true;
  }
}

function wsError(ev) {
  console.error('WebSocket error:', ev);
  reset();
}

function wsOpen() {
  console.log('WebSocket open');
}

function wsClose() {
  console.log('Socket closed');
  ws = null;
  recording = null;
  closing = false;
}

class BadMessage extends Error {}

function handleStart(data) {
  if (recording != null || closing) {
    return;
  }
  const { width, height, framerate, length } = data;
  if (typeof width != 'number') throw new BadMessage('Missing width');
  if (typeof height != 'number') throw new BadMessage('Missing height');
  if (typeof framerate != 'number') throw new BadMessage('Missing framerate');
  if (typeof length != 'number') throw new BadMessage('Missing length');
  recording = {
    width,
    height,
    framerate,
    length,
    pos: 0,
    ready: false,
    acked: 0,
  };
}

function handleAck(data) {
  if (recording == null || closing) {
    return;
  }
  const { frame } = data;
  if (typeof frame != 'number') throw new BadMessage('Missing frame');
  recording.acked = frame;
}

function handleMessage(data) {
  if (typeof data != 'string') {
    throw new BadMessage('Not a string');
  }
  let obj;
  try {
    obj = JSON.parse(data);
  } catch (e) {
    throw new BadMessage('Invalid JSON');
  }
  const { type } = obj;
  if (typeof type != 'string') {
    throw new BadMessage('Missing type field');
  }
  switch (type) {
    case 'start':
      handleStart(obj);
      break;
    case 'ack':
      handleAck(obj);
      break;
    default:
      throw new BadMessage('Unknown message type');
  }
}

function wsMessage(ev) {
  const { data } = ev;
  try {
    handleMessage(data);
  } catch (e) {
    if (e instanceof BadMessage) {
      console.error('Could not parse message', e, data);
    } else {
      console.error(e);
    }
    ws.close();
    ws = null;
    recording = null;
  }
}

export async function startRecording() {
  if (ws != null) {
    return;
  }
  ws = new WebSocket(`ws://${window.location.host}/__wscapture__/socket`);
  ws.addEventListener('error', wsError);
  ws.addEventListener('open', wsOpen);
  ws.addEventListener('close', wsClose);
  ws.addEventListener('message', wsMessage);
}

export function stopRecording() {
  if (ws == null || closing) {
    return;
  } else if (recording == null) {
    ws.close();
    ws = null;
  } else {
    ws.send(new Uint8Array(0));
    recording = null;
    closing = true;
  }
}

// Get the current time in the recording.
export function currentTimeMS(time) {
  if (recording == null) {
    return time;
  }
  return (1000 * recording.pos) / recording.framerate;
}

// Call before rendering a frame. Returns true if the frame should be rendered,
// false if the frame should be skipped.
export function beginFrame() {
  if (recording == null) {
    return ws == null;
  }
  const gl = glContext;
  if (gl == null) {
    console.warn('No GL context.');
    return false;
  }
  const { canvas, drawingBufferWidth, drawingBufferHeight } = gl;
  recording.ready = false;
  const { width, height } = recording;
  if (width != drawingBufferWidth || height != drawingBufferHeight) {
    console.log(
      `Resizing canvas from ${drawingBufferWidth}x${drawingBufferHeight} to ${width}x${height}`,
    );
    canvas.width = width;
    canvas.height = height;
    return false;
  }
  if (recording.pos - recording.acked > maxOutstanding) {
    return false;
  }
  recording.ready = true;
  return true;
}

// Call after a frame has been rendered.
export function endFrame() {
  if (recording == null || !recording.ready) {
    return;
  }
  const gl = glContext;
  if (gl == null) {
    return;
  }
  const { width, height } = recording;
  const buffer = new Uint8Array(width * height * 4);
  gl.readPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, buffer);
  ws.send(buffer);
  recording.pos++;
  if (recording.length >= 0 && recording.pos >= recording.length) {
    console.log(`Done: length=${recording.length}; pos=${recording.pos}`);
    stopRecording();
  }
}
