-- Uses sources written for blink.cmp as goda-cmp sources (the Go side is
-- package luasrc).
--
-- Many blink.cmp sources (pane, ghq, ripgrep, emoji, nerdfont, dictionary,
-- digraphs, ...) depend on little more than the table of kinds and a small
-- async library of blink.cmp. blink.cmp does not have to be set up: it is
-- enough that its modules can be required, so the directories of the plugins
-- are added to package.path (the paths of setup()).
--
-- The context given to a source is a minimal imitation of that of blink.cmp
-- (line, cursor, bounds, trigger). The candidates are in the shape of an LSP
-- CompletionItem, so they are converted with goda_cmp_luasrc.convert() as in
-- nvimlsp.

local M = {}

local luasrc = require "goda_cmp_luasrc"

---@class goda_cmp_luasrc.blink.Spec
---@field module string the module of the blink.cmp source
---@field opts? table the opts of the source (providers.*.opts of blink.cmp)
---@field static? boolean the candidates do not change with the typed text: do not ask again while typing on
---@field insert_text? boolean insert insertText instead of newText (goda_cmp_luasrc.convert)
---@field transform? fun(ctx: table, items: lsp.CompletionItem[]): lsp.CompletionItem[] transform_items of blink.cmp
---@field icon? fun(kind: string?): string?, string? an icon and its highlight per candidate (the state of an issue or a PR of git, etc.)
---@field at_trigger? boolean make the cursor look right after the trigger character (context())

---@type table<string, goda_cmp_luasrc.blink.Spec>
local specs = {}
local instances = {}

--- @param opts { paths?: string[], sources: table<string, goda_cmp_luasrc.blink.Spec> }
function M.setup(opts)
  for _, dir in ipairs(opts.paths or {}) do
    package.path = ("%s/lua/?.lua;%s/lua/?/init.lua;%s"):format(dir, dir, package.path)
  end
  specs = opts.sources
end

local function instance(name)
  if instances[name] == nil then
    local spec = specs[name]
    local ok, src = pcall(function()
      return require(spec.module).new(vim.deepcopy(spec.opts or {}), { name = name })
    end)
    if not ok then
      vim.notify(("goda_cmp_luasrc.blink: %s: %s"):format(name, src), vim.log.levels.ERROR)
    end
    instances[name] = ok and src or false
  end
  return instances[name] or nil
end

-- context.trigger of blink.cmp. If the character before the word is a trigger
-- character of the source, the completion is taken as started by it
-- (blink-cmp-git returns candidates only then).
local function trigger_of(src, a, ks)
  local prev = a.input:sub(ks, ks)
  local chars = src.get_trigger_characters and src:get_trigger_characters() or {}
  if prev ~= "" and vim.tbl_contains(chars, prev) then
    return {
      initial_kind = "trigger_character",
      kind = "trigger_character",
      initial_character = prev,
      character = prev,
    }
  end
  local kind = a.manual and "manual" or "keyword"
  return { initial_kind = kind, kind = kind }
end

local function context(spec, src, a, line, ks)
  local trigger = trigger_of(src, a, ks)
  local col = #a.input
  -- blink-cmp-git makes the range to replace "from one character before the
  -- cursor". That works in blink.cmp, which asks once right after the trigger
  -- character, but goda-cmp may ask after more is typed, so make the cursor
  -- look right after the trigger character.
  if spec.at_trigger and trigger.initial_character then
    col = ks
  end
  return {
    id = a.id,
    mode = "default",
    bufnr = a.bufnr,
    line = line,
    cursor = { a.lnum, col },
    bounds = { line_number = a.lnum, start_col = ks + 1, length = #a.input - ks },
    trigger = trigger,
    providers = {},
    get_keyword = function()
      return a.input:sub(ks + 1)
    end,
    get_line = function()
      return line
    end,
    get_cursor = function()
      return { a.lnum, col }
    end,
  }
end

---@param a { id: integer, source: string, bufnr: integer, lnum: integer, input: string, manual: boolean }
function M.complete(a)
  local spec, src = specs[a.source], instance(a.source)
  local function empty()
    luasrc.reply(a.id, { request = a.id, items = {} })
  end
  if not src or (src.enabled and not src:enabled()) then
    return empty()
  end
  local line = vim.api.nvim_buf_get_lines(a.bufnr, a.lnum - 1, a.lnum, false)[1] or a.input
  -- The default keyword of blink.cmp ([%w_-]). Candidates without a textEdit
  -- replace from here.
  local ks = #a.input - #a.input:match "[%w_%-]*$"
  local conv = { input = a.input, lnum = a.lnum, keywordStart = ks }
  local ft = vim.bo[a.bufnr].filetype
  local ctx = context(spec, src, a, line, ks)
  local replied = false
  -- Some sources call back from async work (a luv callback), so move to where
  -- vim.fn can be used before replying.
  local callback = vim.schedule_wrap(function(resp)
    if replied then
      return
    end
    replied = true
    local list = resp and resp.items or {}
    if spec.transform then
      list = spec.transform(ctx, list)
    end
    -- Look the kind names up in the table of blink.cmp. Sources may add names
    -- to it (openPR of blink-cmp-git, etc.).
    local kinds = require("blink.cmp.types").CompletionItemKind
    local items, entries = {}, {}
    for _, ci in ipairs(list) do
      local it = luasrc.convert(ci, conv, { insert_text = spec.insert_text, filetype = ft })
      if it then
        it.kind = kinds[ci.kind]
        -- The score offset per candidate (blink-cmp-git expresses the order of
        -- its list with it, and transform_items may use it too).
        it.scoreOffset = ci.score_offset
        if spec.icon then
          it.icon, it.iconGroup = spec.icon(it.kind)
        end
        table.insert(entries, { item = ci, filetype = ft })
        it.index = #entries
        table.insert(items, it)
      end
    end
    luasrc.remember(a, entries)
    luasrc.reply(a.id, {
      request = a.id,
      items = items,
      incomplete = not resp or resp.is_incomplete_forward ~= false,
      cacheable = spec.static or (resp ~= nil and resp.is_incomplete_forward == false),
    })
  end)
  local ok, err = pcall(src.get_completions, src, ctx, callback)
  if not ok then
    vim.notify(("goda_cmp_luasrc.blink: %s: %s"):format(a.source, err), vim.log.levels.ERROR)
    replied = true
    empty()
  end
end

---@param a { id: integer, source: string, request: integer, index: integer }
function M.resolve(a)
  local e, src = luasrc.entry(a), instance(a.source)
  if not e or not src or not src.resolve then
    return luasrc.reply(a.id, vim.empty_dict())
  end
  local replied = false
  local ok = pcall(
    src.resolve,
    src,
    e.item,
    vim.schedule_wrap(function(ci)
      if replied then
        return
      end
      replied = true
      luasrc.reply(a.id, { info = ci and luasrc.info(ci, e.filetype) or "" })
    end)
  )
  if not ok and not replied then
    replied = true
    luasrc.reply(a.id, vim.empty_dict())
  end
end

-- Nothing to do on confirmation (convert has replaced the emoji, etc.).
function M.done() end

return M
