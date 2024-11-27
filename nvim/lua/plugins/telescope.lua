return {
  {
    "nvim-telescope/telescope.nvim",
    opts = function()
      return {
        defaults = {
          require("telescope").setup({
            pickers = {
              find_files = {
                find_command = { "rg", "--files", "--sortr=modified" },
              },
            },
          }),
          -- When I search for stuff in telescope, I want the path to be shown
          -- first, this helps in files that are very deep in the tree and I
          -- cannot see their name.
          -- Also notice the "reverse_directories" option which will show the
          -- closest dir right after the filename
          path_display = {
            filename_first = {
              reverse_directories = true,
            },
          },
        },
      }
    end,
    keys = {
      { "<leader>fz", LazyVim.pick("auto", { root = false }), desc = "Find Files (Root Dir)" },
      { "<leader>fF", "<cmd>Telescope frecency<cr>", desc = "Find Files (cwd)" },
      {
        "<leader>ff",
        "<cmd>Telescope frecency workspace=CWD theme=ivy<cr>",
        desc = "Find Files (Root Dir)",
      },
      { "<leader>sG", LazyVim.pick("live_grep"), desc = "Grep (cwd)" },
      { "<leader>sg", LazyVim.pick("live_grep", { root = false }), desc = "Grep (Root Dir)" },
      {
        "<leader><space>",
        "<cmd>Telescope frecency workspace=CWD theme=ivy<cr>",
        desc = "Find Files (Root Dir)",
      },
    },
  },
}
