import {
  Card,
  Grid,
  Group,
  SimpleGrid,
  Skeleton,
  Stack,
  Table,
} from "@mantine/core";

export function DashboardSkeleton() {
  return (
    <Stack>
      <Skeleton height={28} width={120} />

      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
        {Array.from({ length: 4 }, (_, i) => (
          <Card withBorder padding="lg" radius="md" key={i}>
            <Group justify="space-between">
              <div>
                <Skeleton height={12} width={80} mb={8} />
                <Skeleton height={24} width={60} />
              </div>
              <Skeleton height={28} width={28} radius="sm" />
            </Group>
          </Card>
        ))}
      </SimpleGrid>

      <Grid>
        {Array.from({ length: 4 }, (_, i) => (
          <Grid.Col span={{ base: 12, md: 6 }} key={i}>
            <Card withBorder padding="lg" radius="md">
              <Skeleton height={16} width={100} mb="sm" />
              <Stack gap={4}>
                {Array.from({ length: 4 }, (_, j) => (
                  <Group justify="space-between" key={j}>
                    <Skeleton height={14} width={90} />
                    <Skeleton height={14} width={50} />
                  </Group>
                ))}
              </Stack>
            </Card>
          </Grid.Col>
        ))}
      </Grid>
    </Stack>
  );
}

export function ListPageSkeleton() {
  return (
    <Stack>
      <Group justify="space-between">
        <Skeleton height={28} width={120} />
        <Group gap="xs">
          <Skeleton height={36} width={36} radius="sm" />
          <Skeleton height={36} width={36} radius="sm" />
        </Group>
      </Group>

      <Group gap="sm">
        <Skeleton height={36} width={240} radius="sm" />
        <Skeleton height={36} width={120} radius="sm" />
      </Group>

      <Table>
        <Table.Thead>
          <Table.Tr>
            {Array.from({ length: 5 }, (_, i) => (
              <Table.Th key={i}>
                <Skeleton height={14} width={60 + i * 20} />
              </Table.Th>
            ))}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {Array.from({ length: 8 }, (_, i) => (
            <Table.Tr key={i}>
              {Array.from({ length: 5 }, (_, j) => (
                <Table.Td key={j}>
                  <Skeleton height={14} width={50 + j * 15} />
                </Table.Td>
              ))}
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>

      <Group justify="center">
        <Skeleton height={32} width={200} radius="sm" />
      </Group>
    </Stack>
  );
}

export function DetailPageSkeleton() {
  return (
    <Stack>
      <Skeleton height={14} width={200} />

      <Group justify="space-between">
        <Skeleton height={28} width={180} />
        <Skeleton height={36} width={80} radius="sm" />
      </Group>

      <Card withBorder padding="lg" radius="md">
        <SimpleGrid cols={{ base: 1, sm: 2 }}>
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i}>
              <Skeleton height={12} width={70} mb={4} />
              <Skeleton height={16} width={120 + i * 10} />
            </div>
          ))}
        </SimpleGrid>
      </Card>

      <Group gap="md">
        <Skeleton height={36} width={100} radius="sm" />
        <Skeleton height={36} width={100} radius="sm" />
      </Group>

      <Table>
        <Table.Thead>
          <Table.Tr>
            {Array.from({ length: 4 }, (_, i) => (
              <Table.Th key={i}>
                <Skeleton height={14} width={60 + i * 20} />
              </Table.Th>
            ))}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {Array.from({ length: 5 }, (_, i) => (
            <Table.Tr key={i}>
              {Array.from({ length: 4 }, (_, j) => (
                <Table.Td key={j}>
                  <Skeleton height={14} width={50 + j * 15} />
                </Table.Td>
              ))}
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

export function CardPageSkeleton() {
  return (
    <Stack>
      <Skeleton height={28} width={120} />

      {Array.from({ length: 3 }, (_, i) => (
        <Card withBorder padding="lg" radius="md" key={i}>
          <Skeleton height={16} width={100 + i * 20} mb="sm" />
          <Stack gap="xs">
            {Array.from({ length: 3 + i }, (_, j) => (
              <Group justify="space-between" key={j}>
                <Skeleton height={14} width={100} />
                <Skeleton height={14} width={140} />
              </Group>
            ))}
          </Stack>
        </Card>
      ))}
    </Stack>
  );
}
