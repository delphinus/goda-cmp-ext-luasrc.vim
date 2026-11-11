-- Draws the information of a candidate (the Markdown goda_cmp_luasrc.info()
-- makes) in the preview window of pum.vim. The Preview of the source luasrc
-- (luasrc/luasrc.go) gives this as the command of the preview. It runs in the
-- window, so the current buffer and window are those of the preview.
--
-- pum.vim only sets the filetype to markdown, which gives the colors of the
-- legacy syntax/markdown.vim only. To look like the documentation window of
-- blink.cmp, draw it with the treesitter markdown parser (code blocks are
-- colored in their languages and the ``` lines are concealed), and draw the
-- separator --- as a line. The same as the LSP hover window of Neovim does.

local M = {}

local ns = vim.api.nvim_create_namespace "goda_cmp_luasrc.preview"

function M.render()
  local buf = vim.api.nvim_get_current_buf()
  local win = vim.api.nvim_get_current_win()
  if vim.bo[buf].filetype ~= "markdown" then
    vim.bo[buf].filetype = "markdown"
  end
  pcall(vim.treesitter.start, buf, "markdown")
  local function set_options()
    vim.wo[win].conceallevel = 2
    vim.wo[win].concealcursor = "nvic"
    vim.wo[win].linebreak = true
  end
  set_options()
  -- pum.vim sets the filetype to markdown again after this command. FileType
  -- fires even for the same value, and a ftplugin may change 'conceallevel', so
  -- set the options once more after that.
  vim.schedule(function()
    if vim.api.nvim_win_is_valid(win) then
      set_options()
    end
  end)

  vim.api.nvim_buf_clear_namespace(buf, ns, 0, -1)
  local width = vim.api.nvim_win_get_width(0)
  for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
    if line == "---" then
      vim.api.nvim_buf_set_extmark(buf, ns, i - 1, 0, {
        virt_text = { { ("─"):rep(width), "FloatBorder" } },
        virt_text_pos = "overlay",
      })
    end
  end
end

return M
