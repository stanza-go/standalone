import { useState } from "react";
import { useNavigate } from "react-router";
import {
  Alert,
  Button,
  Center,
  Container,
  Paper,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { IconAlertCircle } from "@tabler/icons-react";
import { useAuth } from "@/lib/auth";
import { ApiError } from "@/lib/api";

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const form = useForm({
    initialValues: {
      email: "",
      password: "",
    },
    validate: {
      email: (v) => (!v ? "Email is required" : null),
      password: (v) => (!v ? "Password is required" : null),
    },
  });

  async function handleSubmit(values: typeof form.values) {
    setError("");
    setSubmitting(true);
    try {
      await login(values.email, values.password);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
        if (err.fields.email) form.setFieldError("email", err.fields.email);
        if (err.fields.password) form.setFieldError("password", err.fields.password);
      } else {
        setError("Login failed");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Center h="100vh">
      <Container size={420} w="100%">
        <Title ta="center" order={2}>
          Stanza Admin
        </Title>
        <Text c="dimmed" size="sm" ta="center" mt={5}>
          Sign in to continue
        </Text>

        <Paper withBorder shadow="md" p={30} mt={30} radius="md">
          <form onSubmit={form.onSubmit(handleSubmit)}>
            <Stack>
              {error && !form.errors.email && !form.errors.password && (
                <Alert
                  icon={<IconAlertCircle size={16} />}
                  color="red"
                  variant="light"
                >
                  {error}
                </Alert>
              )}

              <TextInput
                label="Email"
                placeholder="admin@stanza.dev"
                autoComplete="email"
                autoFocus
                required
                {...form.getInputProps("email")}
              />

              <PasswordInput
                label="Password"
                placeholder="Your password"
                autoComplete="current-password"
                required
                {...form.getInputProps("password")}
              />

              <Button type="submit" fullWidth loading={submitting}>
                Sign in
              </Button>
            </Stack>
          </form>
        </Paper>
      </Container>
    </Center>
  );
}
