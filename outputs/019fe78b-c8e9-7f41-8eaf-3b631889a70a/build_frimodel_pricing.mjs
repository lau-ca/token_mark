import fs from "node:fs/promises";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const sourceUrl = "https://platform.frimodel.com/pricing";
const collectedAt = "2026-08-10";

const models = [];

function addModel(model, provider, billing, baseSummary, endpoints, groups, notes = "") {
  models.push({ model, provider, billing, baseSummary, endpoints, groups, notes });
}

const tokenTier = (tier, input, output, cacheRead = null, cacheWrite = null, condition = "") => ({
  tier,
  condition,
  unit: "USD / 1M tokens",
  input,
  output,
  cacheRead,
  cacheWrite,
  request: null,
  second: null,
});
const requestTier = (tier, request, condition = "") => ({
  tier,
  condition,
  unit: "USD / 请求",
  input: null,
  output: null,
  cacheRead: null,
  cacheWrite: null,
  request,
  second: null,
});
const secondTier = (tier, second, condition = "") => ({
  tier,
  condition,
  unit: "USD / 秒",
  input: null,
  output: null,
  cacheRead: null,
  cacheWrite: null,
  request: null,
  second,
});
const group = (name, multiplier, tiers) => ({ name, multiplier, tiers });

addModel("claude-fable-5", "Anthropic", "按量计费", "输入 $10；输出 $50；缓存输入 $1；缓存写入 $12.5（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 14, 70, 1.4, 17.5)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 14, 70, 1.4, 17.5)]),
]);
addModel("claude-opus-4-6", "Anthropic", "按量计费", "输入 $5；输出 $25；缓存输入 $0.5；缓存写入 $6.25（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
]);
addModel("claude-opus-4-7", "Anthropic", "按量计费", "输入 $5；输出 $25；缓存输入 $0.5；缓存写入 $6.25（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
]);
addModel("claude-opus-4-8", "Anthropic", "按量计费", "输入 $5；输出 $25；缓存输入 $0.5；缓存写入 $6.25（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
]);
addModel("claude-opus-5", "Anthropic", "按量计费", "输入 $5；输出 $25；缓存输入 $0.5；缓存写入 $6.25（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 7, 35, 0.7, 8.75)]),
]);
addModel("claude-sonnet-4-6", "Anthropic", "按量计费", "输入 $3；输出 $15；缓存输入 $0.3；缓存写入 $3.75（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 4.2, 21, 0.42, 5.25)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 4.2, 21, 0.42, 5.25)]),
]);
addModel("claude-sonnet-5", "Anthropic", "按量计费", "输入 $2；输出 $10；缓存输入 $0.2；缓存写入 $2.5（每 1M tokens）", "anthropic, openai", [
  group("claude_max", 1.4, [tokenTier("标准", 2.8, 14, 0.28, 3.5)]),
  group("claude_max_外接", 1.4, [tokenTier("标准", 2.8, 14, 0.28, 3.5)]),
]);
addModel("codex-auto-review", "OpenAI", "按量计费", "输入 $2；输出 $12；缓存输入 $0.2（每 1M tokens）", "openai", [
  group("default", 0.25, [tokenTier("标准", 0.5, 3, 0.05)]),
]);
addModel("doubao-seedance-2-0-260128", "字节跳动", "动态计费", "$1/秒", "openai", [
  group("Seedance2.0", 1, [secondTier("720p", 1), secondTier("1080p", 1.2)]),
]);
addModel("doubao-seedance-2-0-fast-260128", "字节跳动", "动态计费", "$1/秒", "openai", [
  group("Seedance2.0", 1, [secondTier("per_second", 1)]),
]);
addModel("gemini-3-pro-image-preview", "Google", "按量计费", "输入 $75；输出 $4,500（每 1M tokens）", "gemini, openai", [
  group("gemini_image", 1, [tokenTier("标准", 75, 4500)]),
  group("test", 1, [tokenTier("标准", 75, 4500)]),
]);
addModel("gemini-3.1-flash-image-preview", "Google", "按次计费", "$0.1/请求", "gemini, openai", [
  group("gemini_image", 1, [requestTier("标准", 0.1)]),
  group("test", 1, [requestTier("标准", 0.1)]),
]);
addModel("gpt-5.4", "OpenAI", "动态计费", "输入 $2.5；输出 $15；缓存读取 $0.25（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 0.625, 3.75, 0.0625, null, "Length ≤ 272K"),
    tokenTier("长上下文", 1.25, 5.625, 0.125, null, "Length > 272K"),
  ]),
]);
addModel("gpt-5.4-mini", "OpenAI", "按量计费", "输入 $0.75；输出 $4.5；缓存输入 $0.075（每 1M tokens）", "openai", [
  group("default", 0.25, [tokenTier("标准", 0.1875, 1.125, 0.01875)]),
]);
addModel("gpt-5.4-openai-compact", "OpenAI", "按量计费", "输入 $2.5；输出 $15；缓存输入 $0.25（每 1M tokens）", "openai", [
  group("default", 0.25, [tokenTier("标准", 0.625, 3.75, 0.0625)]),
]);
addModel("gpt-5.5", "OpenAI", "动态计费", "输入 $5；输出 $30；缓存读取 $0.5（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 1.25, 7.5, 0.125, null, "Length ≤ 272K"),
    tokenTier("长上下文", 2.5, 11.25, 0.25, null, "Length > 272K"),
  ]),
]);
addModel("gpt-5.5-openai-compact", "OpenAI", "动态计费", "输入 $5；输出 $30；缓存读取 $0.5（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 1.25, 7.5, 0.125, null, "Length ≤ 272K"),
    tokenTier("长上下文", 2.5, 11.25, 0.25, null, "Length > 272K"),
  ]),
]);
addModel("gpt-5.6-luna", "OpenAI", "动态计费", "输入 $0.75；输出 $4.5；缓存读取 $0.075；缓存写入 $0.75（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 0.1875, 1.125, 0.01875, 0.1875, "Length ≤ 272K"),
    tokenTier("长上下文", 0.1875, 1.125, 0.01875, 0.1875, "Length > 272K"),
  ]),
]);
addModel("gpt-5.6-sol", "OpenAI", "动态计费", "输入 $5；输出 $30；缓存读取 $0.5；缓存写入 $6.25（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 1.25, 7.5, 0.125, 1.5625, "Length ≤ 272K"),
    tokenTier("长上下文", 2.5, 11.25, 0.25, 3.125, "Length > 272K"),
  ]),
]);
addModel("gpt-5.6-terra", "OpenAI", "动态计费", "输入 $2；输出 $12；缓存读取 $0.2；缓存写入 $2.5（每 1M tokens）", "openai", [
  group("default", 0.25, [
    tokenTier("标准上下文", 0.5, 3, 0.05, 0.625, "Length ≤ 272K"),
    tokenTier("长上下文", 1, 4.5, 0.1, 1.25, "Length > 272K"),
  ]),
]);
addModel("gpt-image-2", "OpenAI", "按次计费", "$0.07/请求", "openai", [
  group("codex_image", 0.4, [requestTier("标准", 0.028)]),
  group("gpt_image_web", 0.4, [requestTier("标准", 0.028)]),
]);
addModel("gpt-image-2-adobe", "OpenAI", "按次计费", "$0.07/请求", "openai", [
  group("gpt_image_adobe", 1, [requestTier("标准", 0.07)]),
]);
addModel("gpt-image-2-high", "OpenAI", "按次计费", "$0.07/请求", "openai", [
  group("gpt_image_adobe", 1, [requestTier("标准", 0.07)]),
]);
addModel("gpt-image-2-medium", "OpenAI", "按次计费", "$0.07/请求", "openai", [
  group("gpt_image_adobe", 1, [requestTier("标准", 0.07)]),
]);
addModel("gpt-image-2-w", "OpenAI", "按次计费", "$0.07/请求", "openai", [
  group("codex_image", 0.4, [requestTier("标准", 0.028)]),
  group("gpt_image_web", 0.4, [requestTier("标准", 0.028)]),
]);
addModel("grok-imagine-image", "xAI", "按次计费", "$0.02/请求", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("grok", 1, [requestTier("标准", 0.02)]),
]);
addModel("grok-imagine-image-quality", "xAI", "按次计费", "$0.05/请求", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("grok", 1, [requestTier("标准", 0.05)]),
]);
addModel("grok-imagine-video", "xAI", "按量计费", "输入 $75；输出 $75（每 1M tokens）", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("grok", 1, [tokenTier("标准", 75, 75)]),
]);
addModel("grok-imagine-video-1.5-preview", "xAI", "按次计费", "$0.7/请求", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("grok", 1, [requestTier("标准", 0.7)]),
]);
addModel("Kling 3.0", "快手", "动态计费", "$3/秒", "openai", [
  group("Kling", 1, [
    secondTier("4k", 3), secondTier("pro-sound", 1.2), secondTier("pro", 0.8), secondTier("std-sound", 0.9), secondTier("std", 0.6),
  ]),
]);
addModel("Kling 3.0 Omni", "快手", "动态计费", "$3/秒", "openai", [
  group("Kling", 1, [
    secondTier("4k", 3), secondTier("pro-reference", 1.2), secondTier("pro-sound", 1), secondTier("pro", 0.8), secondTier("std-reference", 0.9), secondTier("std-sound", 0.8), secondTier("std", 0.6),
  ]),
]);
addModel("Kling 3.0 Turbo", "快手", "动态计费", "$1/秒", "openai", [
  group("Kling", 1, [secondTier("1080p", 1), secondTier("720p", 0.8)]),
]);
addModel("seedance-2.5", "页面未标注", "动态计费", "$0.28/秒", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search", [
  group("Seedance2.0", 1, [secondTier("480p", 0.28), secondTier("720p", 0.4)]),
]);
addModel("seedance2.0", "页面未标注", "动态计费", "$3.5/请求", "openai-video", [
  group("Seedance2.0", 1, [requestTier("per_request", 3.5)]),
]);
addModel("tvideos", "页面未标注", "动态计费", "$5.5/请求", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search", [
  group("Seedance2.0", 1, [
    requestTier("480p", 5.5), requestTier("720p", 8), secondTier("1080p", 1), secondTier("4k", 2),
  ]),
]);
addModel("tvideos-mini", "页面未标注", "动态计费", "$2.5/请求", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search", [
  group("Seedance2.0", 1, [requestTier("480p", 2.5), requestTier("720p", 5)]),
]);
addModel("videos-4", "Doubao", "动态计费", "$0.35/秒", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("Seedance2.0", 1, [secondTier("480p", 0.35), secondTier("720p", 0.5)]),
]);
addModel("videos-4-fast", "Doubao", "动态计费", "$0.2/秒", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("Seedance2.0", 1, [secondTier("480p", 0.2), secondTier("720p", 0.35)]),
]);
addModel("videos-4-mini", "Doubao", "动态计费", "$0.15/秒", "openai, openai-response, openai-response-compact, anthropic, gemini, openai-alpha-search, openai-video", [
  group("Seedance2.0", 1, [secondTier("480p", 0.15), secondTier("720p", 0.25)]),
]);

if (models.length !== 39) throw new Error(`模型数量应为 39，实际为 ${models.length}`);

function formatCondition(condition) {
  if (!condition) return "";
  if (condition === "Length < 272K") return "上下文少于 272K";
  if (condition === "Length > 272K") return "上下文超过 272K";
  if (condition === "Length ≥ 272K") return "上下文达到或超过 272K";
  if (condition === "Length ≤ 272K") return "上下文不超过 272K";
  return condition;
}

function formatPrice(tier, model, includeTierName) {
  const parts = [];
  if (tier.input != null) parts.push(`输入 $${tier.input}/1M`);
  if (tier.output != null) parts.push(`输出 $${tier.output}/1M`);
  if (tier.cacheRead != null) {
    const cacheLabel = model.provider === "Anthropic" ? "缓存输入" : "缓存读取";
    parts.push(`${cacheLabel} $${tier.cacheRead}/1M`);
  }
  if (tier.cacheWrite != null) parts.push(`缓存写入 $${tier.cacheWrite}/1M`);
  if (tier.request != null) parts.push(`$${tier.request}/次`);
  if (tier.second != null) parts.push(`$${tier.second}/秒`);

  let tierName = tier.tier;
  if (tierName === "per_second" || tierName === "per_request" || tierName === "标准") tierName = "";
  const condition = formatCondition(tier.condition);
  if (condition) tierName = `${tierName}（${condition}）`;
  return includeTierName && tierName ? `${tierName}：${parts.join("；")}` : parts.join("；");
}

function billingMethod(tier) {
  if (tier.request != null) return "按次";
  if (tier.second != null) return "按秒";
  return "按量";
}

const groupOrder = [
  "Kling",
  "Seedance2.0",
  "claude_max",
  "claude_max_外接",
  "codex_image",
  "default",
  "gemini_image",
  "gpt_image_adobe",
  "gpt_image_web",
  "grok",
  "test",
];

const pricingRows = [];
for (const groupName of groupOrder) {
  const groupModels = models
    .filter((model) => model.groups.some((priceGroup) => priceGroup.name === groupName))
    .sort((left, right) => left.model.localeCompare(right.model, "en"));

  for (const model of groupModels) {
    const priceGroup = model.groups.find((item) => item.name === groupName);
    const tiersByMethod = new Map();
    for (const tier of priceGroup.tiers) {
      const method = billingMethod(tier);
      const tiers = tiersByMethod.get(method) ?? [];
      tiers.push(tier);
      tiersByMethod.set(method, tiers);
    }

    for (const [method, tiers] of tiersByMethod) {
      const includeTierName = tiers.length > 1 || tiers.some((tier) => tier.condition);
      pricingRows.push([
        "",
        groupName,
        priceGroup.multiplier,
        model.model,
        tiers.map((tier) => formatPrice(tier, model, includeTierName)).join("\n"),
        method,
      ]);
    }
  }
}

if (pricingRows.length !== 50) {
  throw new Error(`聚合价格行数应为 50，实际为 ${pricingRows.length}`);
}

const workbook = Workbook.create();
const pricing = workbook.worksheets.add("FriModel 定价表");
pricing.showGridLines = false;

const headers = ["来源", "分组", "分组倍率", "模型", "模型价格", "计费方式"];
pricing.getRange("A1:F1").values = [headers];
pricing.getRange(`A2:F${1 + pricingRows.length}`).values = pricingRows;

const table = pricing.tables.add(`A1:F${1 + pricingRows.length}`, true, "FriModelPricingTable");
table.style = "TableStyleMedium2";
table.showFilterButton = true;

pricing.freezePanes.freezeRows(1);
pricing.getRange(`A1:F${1 + pricingRows.length}`).format.verticalAlignment = "center";
pricing.getRange(`A2:F${1 + pricingRows.length}`).format.wrapText = true;
for (let index = 0; index < pricingRows.length; index += 1) {
  const priceText = pricingRows[index][4];
  const visualLineCount = priceText
    .split("\n")
    .reduce((total, line) => total + Math.max(1, Math.ceil([...line].length / 62)), 0);
  pricing.getRange(`A${index + 2}:F${index + 2}`).format.rowHeight = Math.max(34, visualLineCount * 18 + 8);
}
pricing.getRange(`C2:C${1 + pricingRows.length}`).format.numberFormat = '0.##"x"';
pricing.getRange(`C2:C${1 + pricingRows.length}`).format.horizontalAlignment = "center";
pricing.getRange(`F2:F${1 + pricingRows.length}`).format.horizontalAlignment = "center";
pricing.getRange(`D2:D${1 + pricingRows.length}`).format.font = { bold: true, color: "#0F4C5C" };
pricing.getRange(`E2:E${1 + pricingRows.length}`).format.font = { color: "#0F766E" };
pricing.getRange(`A2:A${1 + pricingRows.length}`).format.fill = "#FFF7CC";
pricing.getRange(`A1:F${1 + pricingRows.length}`).format.borders = {
  insideHorizontal: { style: "thin", color: "#D7E0E8" },
  bottom: { style: "thin", color: "#94A3B8" },
};

pricing.getRange("A:A").format.columnWidth = 18;
pricing.getRange("B:B").format.columnWidth = 24;
pricing.getRange("C:C").format.columnWidth = 13;
pricing.getRange("D:D").format.columnWidth = 34;
pricing.getRange("E:E").format.columnWidth = 76;
pricing.getRange("F:F").format.columnWidth = 14;

pricing.getRange(`C2:C${1 + pricingRows.length}`).conditionalFormats.add("cellIs", {
  operator: "lessThan",
  formula: 1,
  format: { fill: "#DCFCE7", font: { color: "#166534", bold: true } },
});
pricing.getRange(`C2:C${1 + pricingRows.length}`).conditionalFormats.add("cellIs", {
  operator: "greaterThan",
  formula: 1,
  format: { fill: "#FEF3C7", font: { color: "#92400E", bold: true } },
});

const outputDir = "/Users/mini/develop/project/new-api/outputs/019fe78b-c8e9-7f41-8eaf-3b631889a70a";
await fs.mkdir(outputDir, { recursive: true });

const pricingPreview = await workbook.render({ sheetName: "FriModel 定价表", range: `A1:F${1 + pricingRows.length}`, scale: 1.1, format: "png" });
await fs.writeFile(`${outputDir}/FriModel-定价表-preview.png`, new Uint8Array(await pricingPreview.arrayBuffer()));

const pricingCheck = await workbook.inspect({
  kind: "table",
  range: `FriModel 定价表!A1:F${1 + pricingRows.length}`,
  include: "values,formulas",
  tableMaxRows: 60,
  tableMaxCols: 6,
});
const seedanceCheck = await workbook.inspect({
  kind: "match",
  searchTerm: "seedance-2.5",
  options: { maxResults: 10 },
  summary: "seedance-2.5 tier check",
});
const errors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 300 },
  summary: "final formula error scan",
});

const outputPath = `${outputDir}/FriModel 定价表.xlsx`;
const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputPath);

console.log(JSON.stringify({
  outputPath,
  modelCount: models.length,
  pricingRowCount: pricingRows.length,
  pricingCheck: pricingCheck.ndjson,
  seedanceCheck: seedanceCheck.ndjson,
  errors: errors.ndjson,
}, null, 2));
