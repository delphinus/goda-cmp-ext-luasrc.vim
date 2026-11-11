-- Types the demo into demo/sample/main.go, then quits (GODA_DEMO_DRIVE=1).

local steps = {
  { 1500, "G" },
  { 300, "O" },
}
for c in ("fmt.Pr"):gmatch "." do
  table.insert(steps, { 180, c })
end
vim.list_extend(steps, {
  { 1800, "<C-n>" },
  { 1500, "<C-n>" },
  { 1500, "<C-n>" },
  { 1800, "<C-y>" },
  { 300, "(" },
  { 120, "w" },
  { 120, "o" },
  { 120, "r" },
  { 120, "d" },
  { 120, "s" },
  { 120, ")" },
  { 1000, "<Esc>" },
})

local function feed(keys)
  vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes(keys, true, false, true), "t", false)
end

local function run(i)
  if i > #steps then
    vim.defer_fn(function()
      if vim.env.GODA_DEMO_LOG then
        vim.fn.writefile(vim.fn["goda#log"](), vim.env.GODA_DEMO_LOG)
      end
      vim.cmd "qa!"
    end, 1500)
    return
  end
  vim.defer_fn(function()
    feed(steps[i][2])
    run(i + 1)
  end, steps[i][1])
end

-- Start once gopls has attached.
vim.wait(10000, function()
  return #vim.lsp.get_clients { bufnr = 0, name = "gopls" } > 0
end, 100)
-- Clear what startup said (the recording terminal answers no queries).
vim.cmd "echo '' | redraw"
run(1)
