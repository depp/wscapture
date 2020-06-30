import {
  setContext,
  startRecording,
  stopRecording,
  currentTimeMS,
  beginFrame,
  endFrame,
} from '/__wscapture__/module.js';

const texWidth = 256,
  texHeight = 64;
const canvasWidth = 640,
  canvasHeight = 480;

let errBox = null;

function handleError(msg) {
  if (errBox == null) {
    errBox = document.createElement('div');
    errBox.clasName = 'error';
    document.body.appendChild(errBox);
  }
  const p = document.createElement('p');
  p.appendChild(document.createTextNode(msg));
  errBox.appendChild(p);
}

function handleException(e) {
  handleError(`Exception: ${e.__proto__.name}: ${e.message}`);
}

function makeButton(name, callback) {
  const elt = document.getElementById(name);
  if (!(elt instanceof HTMLButtonElement)) {
    handleError(`Not a button: ${name}`);
    return;
  }
  elt.disabled = false;
  elt.addEventListener('click', callback);
}

let stopFunc = null;

const vertSource1 = `#version 100
attribute vec4 Data;
varying vec2 vTexCoord;
void main() {
  vTexCoord = Data.zw;
  gl_Position = vec4(Data.xy, 0.0, 1.0);
}
`;

const fragSource1 = `#version 100
precision mediump float;
varying vec2 vTexCoord;
uniform sampler2D Texture;
void main() {
  gl_FragColor = texture2D(Texture, vTexCoord);
}
`;

const vertSource2 = `#version 100
attribute vec2 Position;
attribute vec4 Color;
varying vec4 vColor;
void main() {
  vColor = Color;
  gl_Position = vec4(Position, 0.0, 1.0);
}
`;

const fragSource2 = `#version 100
precision lowp float;
varying vec4 vColor;
void main() {
  gl_FragColor = vColor;
}
`;

function start() {
  if (stopFunc != null) {
    return;
  }

  const parent = document.getElementById('canvas');
  if (!(parent instanceof HTMLDivElement)) {
    throw new Error('Cannot find canvas parent');
  }

  const tcanvas = document.createElement('canvas');
  tcanvas.width = texWidth;
  tcanvas.height = texHeight;
  const ctx = tcanvas.getContext('2d');
  if (ctx == null) {
    throw new Error('Could not create 2D context');
  }

  const gcanvas = document.createElement('canvas');
  gcanvas.width = canvasWidth;
  gcanvas.height = canvasHeight;
  const gl = gcanvas.getContext('webgl', { alpha: false });
  if (gl == null) {
    throw new Error('Could not create GL context');
  }
  setContext(gl);
  gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true);

  parent.appendChild(gcanvas);
  const textVerts = 4;
  const shapeVerts = 4 + 5 + 6;
  const arr = new ArrayBuffer(textVerts * 16 + shapeVerts * 12);
  let offset = 0;
  function nextSlice(t, n) {
    const r = new t(arr, offset, n);
    offset += n * t.BYTES_PER_ELEMENT;
    return r;
  }
  const textArr = nextSlice(Float32Array, textVerts * 4);
  const posArr = nextSlice(Float32Array, shapeVerts * 2);
  const colorArr = nextSlice(Uint32Array, shapeVerts);
  let handle = null;
  const buf = gl.createBuffer();

  const program1 = linkProgram(
    ['Data'],
    compileShader(gl.VERTEX_SHADER, vertSource1),
    compileShader(gl.FRAGMENT_SHADER, fragSource1),
  );
  const program2 = linkProgram(
    ['Position', 'Color'],
    compileShader(gl.VERTEX_SHADER, vertSource2),
    compileShader(gl.FRAGMENT_SHADER, fragSource2),
  );
  const uTexture = gl.getUniformLocation(program1, 'Texture');
  const texture = gl.createTexture();

  function compileShader(shaderType, source) {
    const shader = gl.createShader(shaderType);
    if (shader == null) {
      throw new Error('Could not create shader');
    }
    gl.shaderSource(shader, source);
    gl.compileShader(shader);
    const log = gl.getShaderInfoLog(shader);
    if (log != null && log != '') {
      console.log('Info log:', log);
    }
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      gl.deleteShader(shader);
      throw new Error('Compilation failed');
    }
    return shader;
  }

  function linkProgram(attrs, ...shaders) {
    const program = gl.createProgram();
    if (program == null) {
      throw new Error('Could not create program');
    }
    for (const shader of shaders) {
      gl.attachShader(program, shader);
    }
    for (let i = 0; i < attrs.length; i++) {
      gl.bindAttribLocation(program, i, attrs[i]);
    }
    gl.linkProgram(program);
    const log = gl.getProgramInfoLog(program);
    if (log != null && log != '') {
      console.log('Info log:', log);
    }
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      gl.deleteProgram(program);
      throw new Error('Could not link program');
    }
    return program;
  }

  function drawTexture(t) {
    ctx.save();
    ctx.clearRect(0, 0, texWidth, texHeight);
    ctx.fillStyle = '#fff';
    ctx.font = 'bold 20px sans-serif';
    ctx.fillText(`Time: ${(t * 1e-3).toFixed(3)}s`, 10, texHeight - 10);
    ctx.restore();
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texImage2D(
      gl.TEXTURE_2D,
      0,
      gl.RGBA,
      gl.RGBA,
      gl.UNSIGNED_BYTE,
      tcanvas,
    );
    gl.generateMipmap(gl.TEXTURE_2D);
    gl.bindTexture(gl.TEXTURE_2D, null);
  }

  function animate(timeMS) {
    if (!beginFrame()) {
      handle = requestAnimationFrame(animate);
      return;
    }
    const t = currentTimeMS(timeMS);

    const { drawingBufferWidth, drawingBufferHeight } = gl;
    let xscale = 1,
      yscale = 1;
    if (drawingBufferWidth > drawingBufferHeight) {
      xscale = drawingBufferHeight / drawingBufferWidth;
    } else {
      yscale = drawingBufferWidth / drawingBufferHeight;
    }

    gl.clearColor(0.1, 0.1, 0.1, 1.0);
    gl.clear(gl.COLOR_BUFFER_BIT);

    drawTexture(t);
    {
      const x0 = -1.0;
      const x1 = -1.0 + (2.0 * texWidth) / drawingBufferWidth;
      const y0 = -1.0;
      const y1 = -1.0 + (2.0 * texHeight) / drawingBufferHeight;
      // prettier-ignore
      textArr.set([
        x0, y0, 0, 1,
        x1, y0, 1, 1,
        x0, y1, 0, 0,
        x1, y1, 1, 0,
      ]);
    }
    {
      let offset = 0;
      const xs = xscale * 0.25;
      const ys = yscale * 0.25;
      for (let i = 0; i < 3; i++) {
        const phase = (t / (1500 - 200 * i)) % 1;
        let x0 = xscale * 0.5 * (i - 1);
        let y0 = yscale * 0.5 * Math.sin(2.0 * Math.PI * phase);
        const rotation = phase;
        const n = i + 3;
        posArr[offset * 2] = x0;
        posArr[offset * 2 + 1] = y0;
        for (let j = 0; j < n; j++) {
          const a = 2 * Math.PI * (rotation + j / n);
          posArr[(offset + j) * 2] = x0 + xs * Math.sin(a);
          posArr[(offset + j) * 2 + 1] = y0 + ys * Math.cos(a);
        }
        const color = [0x2234d6, 0x16bf13, 0xd96443][i];
        colorArr.fill(color, offset, offset + n);
        offset += n;
      }
    }

    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, arr, gl.STREAM_DRAW);

    gl.useProgram(program1);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 4, gl.FLOAT, false, 16, 0);
    gl.uniform1i(uTexture, 0);

    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    gl.disable(gl.BLEND);
    gl.bindTexture(gl.TEXTURE_2D, null);

    gl.useProgram(program2);
    gl.enableVertexAttribArray(1);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 8, posArr.byteOffset);
    gl.vertexAttribPointer(
      1,
      4,
      gl.UNSIGNED_BYTE,
      true,
      4,
      colorArr.byteOffset,
    );
    offset = 0;
    for (let i = 0; i < 3; i++) {
      const n = i + 3;
      gl.drawArrays(gl.TRIANGLE_FAN, offset, n);
      offset += n;
    }

    gl.disableVertexAttribArray(0);
    gl.disableVertexAttribArray(1);
    gl.useProgram(null);

    gl.bindBuffer(gl.ARRAY_BUFFER, null);

    endFrame();
    handle = requestAnimationFrame(animate);
  }

  function stop() {
    if (handle != null) {
      cancelAnimationFrame(handle);
      handle = null;
      gcanvas.remove();
      stopFunc = null;
      setContext(null);
    }
  }

  stopFunc = stop;
  handle = requestAnimationFrame(animate);
}

function stop() {
  if (stopFunc != null) {
    stopFunc();
  }
}

function main() {
  makeButton('btn-glstart', start);
  makeButton('btn-glstop', stop);
  makeButton('btn-recstart', startRecording);
  makeButton('btn-recstop', stopRecording);
}

main();
