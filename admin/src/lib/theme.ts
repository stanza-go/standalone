import { createTheme } from "@mantine/core";

export const theme = createTheme({
  primaryColor: "blue",
  fontFamily:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
  defaultRadius: "md",
  components: {
    Button: {
      defaultProps: {
        size: "sm",
      },
    },
    TextInput: {
      defaultProps: {
        size: "sm",
      },
    },
    PasswordInput: {
      defaultProps: {
        size: "sm",
      },
    },
    Select: {
      defaultProps: {
        size: "sm",
      },
    },
    Table: {
      defaultProps: {
        striped: "odd",
        highlightOnHover: true,
        withTableBorder: true,
      },
    },
  },
});
