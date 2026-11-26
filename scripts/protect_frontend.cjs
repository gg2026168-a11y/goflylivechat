const fs = require('fs');
const path = require('path');

const dependencyRoot = process.env.FRONTEND_BUILD_DEPS || path.join(__dirname, 'node_modules');
const JavaScriptObfuscator = require(path.join(dependencyRoot, 'javascript-obfuscator'));
const { minify } = require(path.join(dependencyRoot, 'html-minifier-terser'));
const root = path.resolve(__dirname, '..');
const output = path.join(root, 'dist');
const pages = ['index.html', 'agent.html', 'service.html', 'admin.html'];

function protectJavaScript(source) {
  return JavaScriptObfuscator.obfuscate(source, {
    compact: true,
    controlFlowFlattening: false,
    deadCodeInjection: false,
    debugProtection: false,
    disableConsoleOutput: false,
    numbersToExpressions: true,
    renameGlobals: false,
    selfDefending: false,
    sourceMap: false,
    stringArray: true,
    stringArrayEncoding: ['base64'],
    stringArrayRotate: true,
    stringArrayShuffle: true,
    stringArrayThreshold: 0.75,
    transformObjectKeys: false,
    unicodeEscapeSequence: false
  }).getObfuscatedCode();
}

async function build() {
  fs.rmSync(output, { recursive: true, force: true });
  fs.mkdirSync(output, { recursive: true });
  fs.cpSync(path.join(root, 'assets'), path.join(output, 'assets'), { recursive: true });

  for (const page of pages) {
    const source = fs.readFileSync(path.join(root, page), 'utf8');
    const protectedHtml = source.replace(/(<script(?![^>]*\bsrc=)[^>]*>)([\s\S]*?)(<\/script>)/gi, (match, open, script, close) => {
      if (!script.trim()) return match;
      return open + protectJavaScript(script) + close;
    });
    const result = await minify(protectedHtml, {
      collapseWhitespace: true,
      keepClosingSlash: false,
      minifyCSS: true,
      minifyJS: false,
      removeComments: true,
      removeRedundantAttributes: true,
      removeScriptTypeAttributes: true,
      removeStyleLinkTypeAttributes: true,
      useShortDoctype: true
    });
    fs.writeFileSync(path.join(output, page), result);
    console.log(`${page}: ${Buffer.byteLength(source)} -> ${Buffer.byteLength(result)} bytes`);
  }

  const authPath = path.join(root, 'assets', 'auth.js');
  const protectedAuth = protectJavaScript(fs.readFileSync(authPath, 'utf8'));
  fs.writeFileSync(path.join(output, 'assets', 'auth.js'), protectedAuth);
  console.log(`assets/auth.js: ${fs.statSync(authPath).size} -> ${Buffer.byteLength(protectedAuth)} bytes`);
}

build().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
