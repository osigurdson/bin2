type RegistryCardProps = {
  id: string;
  name: string;
}

export default function RegistryCard(props: RegistryCardProps) {
  return (
    <div><a href={`/dashboard/${props.id}`}>{props.name}</a></div>
  );
}
