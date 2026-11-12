-- The Lua side of the goda-cmp source luasrc (package luasrc), shared by the
-- source modules: handing replies over to Go, keeping the original data of the
-- candidates, and converting candidates in the shape of an LSP CompletionItem.
--
-- Go calls complete(), resolve() and done() of a source module, such as
-- goda_cmp_luasrc.nvimlsp and goda_cmp_luasrc.blink. Both of them produce
-- candidates in the shape of an LSP CompletionItem.

local M = {}

local protocol = vim.lsp.protocol

--- Sends the reply to the request id to Go (the plugin "luasrc", package
--- luasrc explains how). A reply Go stopped waiting for is dropped there.
---@param id integer
function M.reply(id, payload)
  local function send()
    vim.fn["goda#notify"]("luasrc", "reply", id, payload)
  end
  -- A source may call back in a fast event, where Vim functions cannot run.
  if vim.in_fast_event() then
    vim.schedule(send)
  else
    send()
  end
end

-- Request id -> { original data of a candidate, ... }. The oldest ones are
-- dropped, counted per source.
--
-- goda-cmp may reuse a previous result without asking again while the user
-- types on (for example after deleting and retyping an LSP candidate).
-- Meanwhile other sources (a dictionary, etc.) are asked on every character, so
-- counting over all the sources would drop the data of the candidates on the
-- screen first, and they could no longer be resolved.
local entries = {}
local orders = {}
local max_requests = 16

---@param a { id: integer, source: string } the arguments of complete()
function M.remember(a, list)
  entries[a.id] = list
  local order = orders[a.source] or {}
  orders[a.source] = order
  table.insert(order, a.id)
  while #order > max_requests do
    entries[table.remove(order, 1)] = nil
  end
end

---@param a { request: integer, index: integer }
function M.entry(a)
  local list = entries[a.request]
  return list and list[a.index]
end

-- Snippets are not expanded: replace the placeholders with their text.
local function strip_snippet(text)
  local prev
  repeat
    prev = text
    text = text:gsub("%${%d+:([^{}]*)}", "%1")
  until text == prev
  return (text:gsub("%${%d+|([^|,}]*)[^}]*}", "%1"):gsub("%${%d+}", ""):gsub("%$%d+", ""):gsub("\\(.)", "%1"))
end

local function documentation(doc)
  if type(doc) == "table" then
    return doc.value or ""
  end
  return doc or ""
end

--- Makes the information of a candidate Markdown. As in the documentation
--- window of blink.cmp, the detail (a signature, etc.) becomes a code block in
--- the language of the buffer being edited, separated from the documentation
--- (goda_cmp_luasrc.preview draws the separator --- as a line).
---@param ci lsp.CompletionItem
---@param ft? string the filetype of the buffer being edited
function M.info(ci, ft)
  local lines = {}
  local detail = ci.detail or ""
  if detail ~= "" then
    vim.list_extend(lines, { "```" .. (ft or ""), detail, "```" })
  end
  -- Some servers (emmylua_ls) begin and end the documentation with blank lines.
  -- Drop them as the LSP hover of Neovim does (keeping the indent of a line).
  local doc = documentation(ci.documentation):gsub("^%s*\n", ""):gsub("%s+$", "")
  if doc ~= "" then
    if #lines > 0 then
      table.insert(lines, "---")
    end
    table.insert(lines, doc)
  end
  return table.concat(lines, "\n")
end

-- The range of a textEdit. For an InsertReplaceEdit, the insert one.
local function insert_range(edit)
  return edit and (edit.range or edit.insert)
end

-- Turns the highlights of colorful-menu.nvim ({group, range = {start, end}},
-- 0-based byte positions) that overlap [from, to) of the text into
-- {group, start, end} relative to from.
local function spans(highlights, from, to)
  local out = {}
  for _, h in ipairs(highlights) do
    local s, e = math.max(h.range[1], from), math.min(h.range[2], to)
    if h[1] and s < e then
      table.insert(out, { group = h[1], start = s - from, ["end"] = e - from })
    end
  end
  return out
end

--- Builds the label of a candidate with colorful-menu.nvim, if it is installed,
--- as blink.cmp users often do. The layout depends on the language server ls
--- (gopls appends the arguments to the name and puts the result type on the
--- right, etc.), and treesitter colors both the name and the signature in the
--- language.
---
--- colorful-menu pads the whole to max_width, filling the gap before the right
--- part with spaces (\7 and spaces for gopls and others). The width to align
--- the right part to is for the UI to decide from all the candidates, so set
--- max_width to 0 to close the gap (nothing is truncated then), and split the
--- label there into the left and the right parts.
---@return { abbr: string, detail: string, abbrHighlights: table, detailHighlights: table }?
local function label(ci, ls)
  local ok, cm = pcall(require, "colorful-menu")
  if not ok or not ls then
    return nil
  end
  local saved = cm.config.max_width
  cm.config.max_width = 0
  local done, item = pcall(cm.highlights, ci, ls)
  cm.config.max_width = saved
  if not done or type(item) ~= "table" or type(item.text) ~= "string" then
    return nil
  end
  local text, hls = item.text, item.highlights or {}
  local s, e = text:find "\7 *"
  if not s then
    s, e = text:find "   +"
  end
  if not s then
    return { abbr = text, detail = "", abbrHighlights = spans(hls, 0, #text), detailHighlights = {} }
  end
  return {
    abbr = text:sub(1, s - 1),
    detail = text:sub(e + 1),
    abbrHighlights = spans(hls, 0, s - 1),
    detailHighlights = spans(hls, e, #text),
  }
end

--- Converts a CompletionItem into the shape Go takes. nil for one that cannot
--- be inserted (starting on another line, or empty).
---@param ci lsp.CompletionItem
---@param a { input: string, lnum: integer, keywordStart: integer }
---@param opts { defaults?: table, encoding?: string, insert_text?: boolean, filetype?: string, client?: string }
function M.convert(ci, a, opts)
  local defaults = opts.defaults or {}
  local start, text, range = a.keywordStart, nil, nil
  if ci.textEdit then
    range, text = insert_range(ci.textEdit), ci.textEdit.newText
  elseif defaults.editRange then
    range, text = insert_range(defaults.editRange), ci.textEditText or ci.label
  elseif ci.insertText and ci.insertText ~= "" then
    text = ci.insertText
  else
    text = ci.label
  end
  local filter = ci.filterText
  -- Sources that replace newText in resolve on confirmation (emoji, nerdfont)
  -- have the name (:smile:) in newText and the content in insertText. goda-cmp
  -- cannot replace a candidate on confirmation, so insert the content and
  -- filter by the name.
  if opts.insert_text and ci.insertText and ci.insertText ~= "" then
    filter = filter or text
    text = ci.insertText
  end
  if range then
    -- complete() can insert on the cursor line only.
    if range.start.line ~= a.lnum - 1 then
      return nil
    end
    start = vim.str_byteindex(a.input, opts.encoding or "utf-8", range.start.character, false)
  end
  if (ci.insertTextFormat or defaults.insertTextFormat) == protocol.InsertTextFormat.Snippet then
    text = strip_snippet(text)
  end
  text = text:match "^[^\r\n]*"
  if text == "" then
    return nil
  end
  -- Hand the name and the part to put on the right (a signature, etc.) over
  -- separately, for a UI that lays them out and colors them. LSP candidates are
  -- built with colorful-menu.nvim, others put labelDetails.detail on the right.
  local l = label(ci, opts.client)
  if not l then
    local detail = vim.trim((ci.labelDetails and ci.labelDetails.detail or ""):match "^[^\r\n]*")
    l = { abbr = ci.label, detail = detail }
  end
  return {
    word = text,
    abbr = l.abbr ~= text and l.abbr or nil,
    labelDetail = l.detail ~= "" and l.detail or nil,
    abbrHighlights = l.abbrHighlights,
    detailHighlights = l.detailHighlights,
    kind = protocol.CompletionItemKind[ci.kind],
    menu = ci.labelDetails and ci.labelDetails.description or nil,
    info = M.info(ci, opts.filetype),
    filterText = filter,
    sortText = ci.sortText,
    start = start,
  }
end

return M
