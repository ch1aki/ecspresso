local must_env = std.native('must_env');
{
  region: must_env('AWS_REGION'),
  cluster: 'default',
  service: 'test',
  service_definition: 'ecs-service-def.json',
  task_definition: 'ecs-task-def.json',
  timeout: '10m0s',
  plugins: [
    {
      name: 'tfstate',
      config: {
        path: 'terraform.tfstate',
      },
    },
  ],
}
