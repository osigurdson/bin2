export interface Registry {
  id: string;
  name: string;
}

export interface ListRegistriesResponse {
  registries: Registry[];
}
