type RegistryCardProps = {
  name: string;
}

export default function RegistryCard(props: RegistryCardProps) {
  return (
    <div>{props.name}</div>
  );
}
