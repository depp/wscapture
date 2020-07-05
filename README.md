# WSCapture - Capture Video over Web Sockets

WSCapture is a tool for capturing nice, smooth video from a WebGL context. **This tool is a horrible, horrible hack.** It sure produces nice results, though! Rather than simply capturing the output of the screen in real-time, WSCapture captures video offline so you get no jitter and no dropped frames—just smooth video. The video is transferred over WebSocket to FFmpeg for encoding.

Captured video example: [City Knot (demoscene)](https://www.youtube.com/watch?v=asV6yIC_bsk)

- WSCaptuere is a JavaScript library. You must make WSCapture function calls in order to capture video.

- The page you are capturing must be served from local files, served from the WSCapture web server.

- No audio. Do that yourself.

## Requirements

- Go: WSCapture uses a web server written in Go to forward data from the web app to a video encoder.

- FFmpeg: WSCapture pipes the captured video to FFmpeg for encoding.

- Yarn or NPM: (Optional) Necessary if you want to use ES5 (global) modules instead of ES6 modules.

## How to Use

Modify your app to use WSCapture.

1. Load the WSCapture library in your app. If you are using ES6 modules, you can import WSCapture using ES6 import syntax:

    ```javascript
    import * as WSCapture from '/__wscapture__/module.js';
    ```

    Otherwise, you can use the ES5 version of the module, which requires Node and Yarn. First install WSCapture dependencies:

    ```shell
    $ yarn install
    ```

    Then add the ES5 script to your page:

    ```html
    <script src="/__wscapture__/script.js" defer></script>
    ```

1. Call WSCapture functions to start recording.

    - `WSCapture.setContext(gl)`: Uses `gl` as the WebGL context for recording. WSCapture will modify the size of the attached canvas to match the size of the video.

    - `WSCapture.stealContext()`: If you do not have access to the WebGL context, you can call `stealContext()` before the context is created. This will save the result of the next call to `HTMLCanvasElement.getContext()`. This is useful if the WebGL context is hidden inside a library.

    - `WSCapture.startRecording()`: Starts recording a video. This kicks off a request in the background.

    - `WSCapture.stopRecording()`: Stops recording a video.

    - `WSCapture.currentTimeMS(time)`: Returns the current time, relative to the start of the video. If a video is not recording, then this returns the input parameter instead. You must use `currentTimeMS()` to get the current time rather than using the input to your `requestAnimationFrame` callback or `Date.now()`. The `currentTimeMS()` function ensures that your video capture is smooth.

    - `WSCapture.beginFrame()`: Call this function before rendering a WebGL frame. As an optimization, this function will return true if the frame should be rendered, and false if the frame should be skipped. This is just a throttling mechanism so the WebGL context doesn’t get too far ahead of the video encoder.

    `WSCapture.endFrame()`: Call this function after rendering the frame.

1. Run WSCapture.

    ```shell
    $ go build
    $ ./wscapture -root <path-to-app>
    ```

1. Go to `http://localhost:8080/page.html`, where `page.html` is the path to your app’s main HTML page.

## Options



## Example

See `demo/wscapture.js`. You can run the demo app by:

```shell
$ go build
$ ./wscapture -root demo
```

Go to http://localhost:8080/index.html to see the demo.

Press “GL Start” or “GL Stop” to start or stop drawing an animation on-screen using WebGL.

Press “Record Start” or “Record Stop” to start or stop recording video. The video will not record while the WebGL animation is not running, so you must make sure to do that. The videos will appear in the `videos` folder, which is automatically created.
