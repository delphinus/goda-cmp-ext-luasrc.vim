-- A minimal Neovim configuration for the demo: goda.vim, goda-cmp.vim,
-- pum.vim and colorful-menu.nvim from $GODA_DEMO_PLUGINS, this checkout, and
-- gopls. Run by demo/record.sh:
--
--   nvim -u demo/init.lua -i NONE demo/sample/main.go
--
-- colorful-menu.nvim highlights Go with treesitter, so the Go parser has to be
-- in 'runtimepath' (stdpath("data")/site, where nvim-treesitter installs it,
-- is in it by default).

local demo = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h")
local repo = vim.fn.fnamemodify(demo, ":h")
local plugins = assert(vim.env.GODA_DEMO_PLUGINS, "set GODA_DEMO_PLUGINS")
local cache = vim.env.GODA_DEMO_CACHE or (plugins .. "/../cache")

local names = { "goda.vim", "goda-cmp.vim", "pum.vim", "colorful-menu.nvim" }
for _, name in ipairs(names) do
  vim.opt.rtp:prepend(plugins .. "/" .. name)
end
vim.opt.rtp:prepend(repo)

-- Not asked to the terminal: the recording terminal does not answer.
vim.o.background = vim.env.GODA_DEMO_BACKGROUND or "light"
vim.o.swapfile = false
-- Bundled with Neovim 0.12 and later: Catppuccin Latte with a light background
-- (Mocha with a dark one).
pcall(vim.cmd.colorscheme, vim.env.GODA_DEMO_COLORSCHEME or "catppuccin")
vim.o.termguicolors = true
vim.o.number = true
vim.o.signcolumn = "no"
vim.o.laststatus = 0
vim.o.ruler = false
vim.o.showmode = false

-- Build with the plugins above and this checkout (go.work).
vim.g["goda#workspace"] = { plugins .. "/goda.vim", plugins .. "/goda-cmp.vim", repo }
vim.g["goda_cmp#config_dir"] = demo .. "/config"
vim.g["goda_cmp#binary"] = cache .. "/goda-cmp"

-- Snippets are not expanded by nvimlsp.
vim.lsp.config("gopls", {
  cmd = { "gopls" },
  filetypes = { "go" },
  root_markers = { "go.mod" },
  capabilities = { textDocument = { completion = { completionItem = { snippetSupport = false } } } },
  -- Not the packages of the module cache, which differ from one machine to
  -- another.
  settings = { gopls = { completeUnimported = false } },
})
vim.lsp.enable "gopls"

vim.api.nvim_create_autocmd("VimEnter", {
  once = true,
  callback = function()
    vim.fn["pum#set_option"] {
      max_height = 8,
      max_columns = { abbr = 50, kind = 10, menu = 10 },
      preview = true,
      previewer = "goda_cmp#get_previewer",
      preview_delay = 200,
      preview_width = 50,
      preview_height = 8,
    }
    if vim.fn.executable(vim.g["goda_cmp#binary"]) == 0 then
      vim.fn["goda_cmp#build"]()
    end
    vim.fn["goda_cmp#enable"]()
    pcall(vim.treesitter.start)
    if vim.env.GODA_DEMO_DRIVE then
      dofile(demo .. "/drive.lua")
    end
  end,
})

vim.keymap.set("i", "<C-n>", "<Cmd>call pum#map#select_relative(+1)<CR>")
vim.keymap.set("i", "<C-p>", "<Cmd>call pum#map#select_relative(-1)<CR>")
vim.keymap.set("i", "<C-y>", "<Cmd>call pum#map#confirm()<CR>")
vim.keymap.set("i", "<C-e>", "<Cmd>call pum#map#cancel()<CR>")
