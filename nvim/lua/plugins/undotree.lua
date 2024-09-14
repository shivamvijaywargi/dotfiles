return {
  {
    "mbbill/undotree",
    config = function()
      -- Optional: Set any custom settings here
      vim.keymap.set("n", "<leader>tu", ":UndotreeToggle<CR>", { desc = "Toggle UndoTree" })
    end,
  },
}
