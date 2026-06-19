import { copyFileSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { execFileSync, spawnSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const isWindows = process.platform === "win32";
const demoBin = resolve(
  rootDir,
  process.env.DEMO_BIN ?? join("tmp", isWindows ? "irasutoya-demo.exe" : "irasutoya-demo"),
);
const imageDir = join(rootDir, "images");

const locales = [
  {
    code: "en",
    htmlLang: "en",
    searches: ["luffy", "zoro"],
    title: "irasutoya-cli · English demo",
    heading: "native Go CLI",
    previewTitle: "Search image previews",
    previewNote: "Showing image previews for Luffy and Zoro.",
    done: "Done.",
  },
  {
    code: "ja",
    htmlLang: "ja",
    searches: ["ルフィ", "ゾロ"],
    title: "irasutoya-cli · 日本語デモ",
    heading: "Go ネイティブ CLI",
    previewTitle: "検索画像プレビュー",
    previewNote: "ルフィとゾロの画像プレビューを表示しています。",
    done: "完了。",
  },
  {
    code: "zh",
    htmlLang: "zh",
    searches: ["路飞", "索隆"],
    title: "irasutoya-cli · 中文演示",
    heading: "原生 Go CLI",
    previewTitle: "搜索图片预览",
    previewNote: "正在显示路飞和索隆的图片预览。",
    done: "完成。",
  },
  {
    code: "ko",
    htmlLang: "ko",
    searches: ["루피", "조로"],
    title: "irasutoya-cli · 한국어 데모",
    heading: "네이티브 Go CLI",
    previewTitle: "검색 이미지 미리보기",
    previewNote: "루피와 조로 이미지 미리보기를 표시합니다.",
    done: "완료.",
  },
];

function runCli(args) {
  return execFileSync(demoBin, args, {
    cwd: rootDir,
    encoding: "utf8",
    env: { ...process.env, IRASUTOYA_OPEN_IMAGES: "" },
  }).replace(/\r\n/g, "\n");
}

function ffmpeg(args) {
  const result = spawnSync("ffmpeg", args, { cwd: rootDir, stdio: "inherit" });
  if (result.status !== 0) {
    throw new Error(`ffmpeg failed with status ${result.status}`);
  }
}

function summarizeSearch(output) {
  const lines = output.trimEnd().split("\n");
  const secondResult = lines.findIndex((line, index) => index > 0 && line.startsWith("Page URL:"));
  const firstResult = secondResult === -1 ? lines : lines.slice(0, secondResult);
  const visibleImageUrls = 1;
  let imageCount = 0;
  const summarized = [];
  for (const line of firstResult) {
    if (line.startsWith("Image URL:")) {
      imageCount += 1;
      if (imageCount > visibleImageUrls) {
        continue;
      }
    }
    summarized.push(line);
  }
  if (imageCount > visibleImageUrls) {
    summarized.push(`Image URL:   ... ${imageCount - visibleImageUrls} more image URLs`);
  }
  return summarized.join("\n");
}

function buildHtml(locale, commands, previewImages) {
  return String.raw`<!doctype html>
<html lang="__HTML_LANG__">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm/css/xterm.css" />
    <style>
      :root {
        color-scheme: dark;
        font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        background: #141821;
      }

      body {
        margin: 0;
        min-height: 100vh;
        display: grid;
        place-items: center;
        background: #151924;
      }

      .window {
        width: 960px;
        height: 560px;
        overflow: hidden;
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 8px;
        background: #0b1020;
        box-shadow: 0 24px 72px rgba(0, 0, 0, 0.42);
      }

      .bar {
        height: 38px;
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 0 14px;
        background: #202534;
        border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      }

      .dot {
        width: 12px;
        height: 12px;
        border-radius: 999px;
      }

      .red { background: #ff5f57; }
      .yellow { background: #febc2e; }
      .green { background: #28c840; }

      .title {
        margin-left: 10px;
        color: #cbd5e1;
        font-size: 13px;
        font-weight: 600;
      }

      #terminal {
        height: 522px;
        padding: 16px 18px;
        box-sizing: border-box;
      }

      .content {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 252px;
        height: 522px;
      }

      .preview {
        border-left: 1px solid rgba(255, 255, 255, 0.08);
        background: #101623;
        padding: 16px;
        box-sizing: border-box;
      }

      .preview-title {
        color: #d8dee9;
        font-size: 13px;
        font-weight: 700;
        margin-bottom: 12px;
      }

      .grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 10px;
      }

      .thumb {
        aspect-ratio: 1;
        display: grid;
        place-items: center;
        overflow: hidden;
        border: 1px solid rgba(255, 255, 255, 0.1);
        border-radius: 8px;
        background: #f8fafc;
      }

      .thumb img {
        max-width: 88%;
        max-height: 88%;
        object-fit: contain;
        opacity: 0;
        transform: translateY(8px);
        transition: opacity 400ms ease, transform 400ms ease;
      }

      .thumb.visible img {
        opacity: 1;
        transform: translateY(0);
      }

      .xterm {
        height: 100%;
      }
    </style>
  </head>
  <body>
    <main class="window">
      <div class="bar">
        <span class="dot red"></span>
        <span class="dot yellow"></span>
        <span class="dot green"></span>
        <span class="title">__TITLE__</span>
      </div>
      <div class="content">
        <div id="terminal"></div>
        <aside class="preview">
          <div class="preview-title">__PREVIEW_TITLE__</div>
          <div class="grid" id="previewGrid"></div>
        </aside>
      </div>
    </main>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm/lib/xterm.js"></script>
    <script>
      const commands = __COMMANDS__;
      const previewImages = __PREVIEW_IMAGES__;
      const terminal = new Terminal({
        cols: 78,
        rows: 27,
        cursorBlink: true,
        convertEol: true,
        fontFamily: "Cascadia Mono, Consolas, 'Courier New', monospace",
        fontSize: 15,
        lineHeight: 1.18,
        letterSpacing: 0,
        theme: {
          background: "#0b1020",
          foreground: "#d8dee9",
          cursor: "#f8fafc",
          selectionBackground: "#334155",
          brightBlue: "#8ab4f8",
          green: "#34d399",
          yellow: "#facc15",
        },
      });
      terminal.open(document.getElementById("terminal"));
      const previewGrid = document.getElementById("previewGrid");
      for (const src of previewImages) {
        const item = document.createElement("div");
        item.className = "thumb";
        const image = document.createElement("img");
        image.src = src;
        image.alt = "";
        item.appendChild(image);
        previewGrid.appendChild(item);
      }

      const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

      async function typeLine(text) {
        terminal.write("\x1b[32m$\x1b[0m ");
        for (const char of text) {
          terminal.write(char);
          await sleep(42);
        }
        terminal.write("\r\n");
      }

      async function writeOutput(output) {
        for (const line of output.trimEnd().split("\n")) {
          const rendered = line
            .replace(/^Page URL:(.*)$/u, "\x1b[36mPage URL:\x1b[0m$1")
            .replace(/^Title:(.*)$/u, "\x1b[33mTitle:\x1b[0m$1")
            .replace(/^Description:(.*)$/u, "\x1b[35mDescription:\x1b[0m$1")
            .replace(/^Image URL:(.*)$/u, "\x1b[34mImage URL:\x1b[0m$1");
          terminal.write(rendered + "\r\n");
          await sleep(95);
        }
      }

      async function revealPreviews() {
        for (const item of document.querySelectorAll(".thumb")) {
          item.classList.add("visible");
          await sleep(260);
        }
      }

      window.runDemo = async () => {
        terminal.write("\x1b[2J\x1b[H");
        terminal.write("\x1b[1mirasutoya-cli\x1b[0m  __HEADING__\r\n\r\n");
        for (const command of commands) {
          await typeLine(command.input);
          await sleep(320);
          await writeOutput(command.output);
          terminal.write("\r\n");
          await sleep(500);
        }
        await revealPreviews();
        terminal.write("__PREVIEW_NOTE__\r\n\r\n");
        terminal.write("\x1b[32m__DONE__\x1b[0m\r\n");
        await sleep(1400);
      };

      window.__demoReady = true;
    </script>
  </body>
</html>`
    .replace("__HTML_LANG__", locale.htmlLang)
    .replace("__TITLE__", locale.title)
    .replace("__PREVIEW_TITLE__", locale.previewTitle)
    .replace("__COMMANDS__", JSON.stringify(commands))
    .replace("__PREVIEW_IMAGES__", JSON.stringify(previewImages))
    .replace("__HEADING__", locale.heading)
    .replace("__PREVIEW_NOTE__", locale.previewNote)
    .replace("__DONE__", locale.done);
}

async function renderLocale(browser, locale) {
  const rawSearches = locale.searches.map((query) => ({ query, output: runCli(["search", query]) }));
  const commands = rawSearches.map(({ query, output }) => ({
    input: `irasutoya search ${query}`,
    output: summarizeSearch(output),
  }));
  const previewImages = rawSearches.flatMap(({ output }) =>
    output
      .split("\n")
      .filter((line) => line.startsWith("Image URL:"))
      .map((line) => line.replace(/^Image URL:\s*/u, ""))
      .slice(0, 2),
  );
  const videoDir = join(rootDir, "tmp", `demo-video-${locale.code}`);
  const outputPng = join(imageDir, `example-${locale.code}.png`);
  const outputGif = join(imageDir, `demo-${locale.code}.gif`);

  rmSync(videoDir, { recursive: true, force: true });
  mkdirSync(videoDir, { recursive: true });

  const context = await browser.newContext({
    viewport: { width: 1120, height: 700 },
    recordVideo: { dir: videoDir, size: { width: 1120, height: 700 } },
  });
  const page = await context.newPage();
  await page.setContent(buildHtml(locale, commands, previewImages), { waitUntil: "networkidle" });
  await page.waitForFunction(() => window.__demoReady === true);
  await page.waitForFunction(
    () => Array.from(document.images).every((image) => image.complete && image.naturalWidth > 0),
    undefined,
    { timeout: 30000 },
  );
  await page.evaluate(() => window.runDemo());
  await page.locator(".window").screenshot({ path: outputPng });
  await context.close();

  const [videoName] = readdirSync(videoDir).filter((name) => name.endsWith(".webm"));
  if (!videoName) {
    throw new Error(`Playwright did not produce a WebM recording for ${locale.code}`);
  }
  const webmPath = join(videoDir, videoName);
  const palettePath = join(videoDir, "palette.png");
  const gifFilter = "setpts=0.9*PTS,fps=12,scale=960:-1:flags=lanczos";
  ffmpeg(["-y", "-i", webmPath, "-vf", `${gifFilter},palettegen`, palettePath]);
  ffmpeg([
    "-y",
    "-i",
    webmPath,
    "-i",
    palettePath,
    "-lavfi",
    `${gifFilter}[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3`,
    outputGif,
  ]);
  console.log(`Updated ${outputPng}`);
  console.log(`Updated ${outputGif}`);
}

mkdirSync(imageDir, { recursive: true });
const browser = await chromium.launch({ headless: true });
for (const locale of locales) {
  await renderLocale(browser, locale);
}
await browser.close();

copyFileSync(join(imageDir, "demo-en.gif"), join(imageDir, "demo.gif"));
copyFileSync(join(imageDir, "example-en.png"), join(imageDir, "example.png"));
console.log("Updated default English demo aliases");
