-- The Lua side of a goda-cmp source asking the LSP clients of Neovim (the Go
-- side is package luasrc).
--
-- Sends textDocument/completion to the LSP clients of Neovim and returns the
-- reply to Go. The lsp source of goda-cmp-ext starts the servers by
-- itself, so two of each server run with those of Neovim, and the settings
-- have to be written again in Go. This one uses the clients Neovim attached,
-- so one server is enough, and the settings of vim.lsp.config() apply as they
-- are.

local M = {}

local luasrc = require "goda_cmp_luasrc"
local protocol = vim.lsp.protocol

local function context(client, a)
  local provider = client.server_capabilities.completionProvider or {}
  if not a.manual and a.keywordStart == #a.input then
    for _, t in ipairs(provider.triggerCharacters or {}) do
      if t ~= "" and vim.endswith(a.input, t) then
        return { triggerKind = protocol.CompletionTriggerKind.TriggerCharacter, triggerCharacter = t }
      end
    end
  end
  return { triggerKind = protocol.CompletionTriggerKind.Invoked }
end

---@param a { id: integer, source: string, bufnr: integer, lnum: integer, input: string, keywordStart: integer, manual: boolean }
function M.complete(a)
  local bufnr = a.bufnr
  local clients = vim.lsp.get_clients { bufnr = bufnr, method = "textDocument/completion" }
  local items, entries, incomplete = {}, {}, false
  local ft = vim.bo[bufnr].filetype
  local remaining = #clients
  local function finish()
    remaining = remaining - 1
    if remaining <= 0 then
      luasrc.remember(a, entries)
      luasrc.reply(a.id, { request = a.id, items = items, incomplete = incomplete })
    end
  end
  if remaining == 0 then
    remaining = 1
    return finish()
  end
  for _, client in ipairs(clients) do
    local enc = client.offset_encoding
    local params = {
      textDocument = { uri = vim.uri_from_bufnr(bufnr) },
      position = { line = a.lnum - 1, character = vim.str_utfindex(a.input, enc) },
      context = context(client, a),
    }
    local ok = client:request("textDocument/completion", params, function(err, result)
      if not err and result then
        local list = result.items ~= nil and result or { items = result }
        -- Some servers (ctags-lsp) return a JSON null (vim.NIL) in items when
        -- they have no candidates. Passing it to ipairs() would fail here,
        -- finish() would never be reached, and the replies of the other servers
        -- would be dropped on the timeout too.
        if type(list.items) ~= "table" then
          list = { items = {} }
        end
        incomplete = incomplete or list.isIncomplete == true
        local defaults = list.itemDefaults or {}
        for _, ci in ipairs(list.items) do
          local it = luasrc.convert(ci, a, { defaults = defaults, encoding = enc, filetype = ft, client = client.name })
          if it then
            if ci.data == nil then
              ci.data = defaults.data
            end
            table.insert(entries, { client_id = client.id, item = ci, filetype = ft })
            it.index = #entries
            table.insert(items, it)
          end
        end
      end
      finish()
    end, bufnr)
    if not ok then
      finish()
    end
  end
end

local function entry_of(a)
  local e = luasrc.entry(a)
  if not e then
    return
  end
  return e, vim.lsp.get_client_by_id(e.client_id)
end

local function can_resolve(client)
  local provider = client.server_capabilities.completionProvider
  return provider and provider.resolveProvider
end

local function resolve(e, client, bufnr, callback)
  client:request("completionItem/resolve", e.item, function(err, result)
    if not err and result then
      e.item, e.resolved = result, true
    end
    callback(e.item)
  end, bufnr)
end

---@param a { id: integer, request: integer, index: integer }
function M.resolve(a)
  local e, client = entry_of(a)
  if not e or not client then
    return luasrc.reply(a.id, vim.empty_dict())
  end
  -- When the user types on, goda-cmp narrows the previous candidates down
  -- without asking again, and shows them as another list. On the Go side the
  -- candidates of that list are not resolved yet, so return the documentation
  -- even if the candidate has been resolved here.
  if e.resolved or not can_resolve(client) then
    return luasrc.reply(a.id, { info = luasrc.info(e.item, e.filetype) })
  end
  resolve(e, client, nil, function(ci)
    luasrc.reply(a.id, { info = luasrc.info(ci, e.filetype) })
  end)
end

--- Applies the additionalTextEdits (adding an import, etc.) of the confirmed
--- candidate.
---@param a { bufnr: integer, request: integer, index: integer }
function M.done(a)
  local e, client = entry_of(a)
  if not e or not client then
    return
  end
  local function apply(ci)
    local edits = ci.additionalTextEdits
    if edits and #edits > 0 and vim.api.nvim_buf_is_valid(a.bufnr) then
      vim.lsp.util.apply_text_edits(edits, a.bufnr, client.offset_encoding)
    end
  end
  local edits = e.item.additionalTextEdits
  if (not edits or #edits == 0) and not e.resolved and can_resolve(client) then
    resolve(e, client, a.bufnr, apply)
  else
    apply(e.item)
  end
end

return M
