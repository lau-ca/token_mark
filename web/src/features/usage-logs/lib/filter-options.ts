export type LogFilterGroup = {
  name: string;
  composite?: boolean;
  publicModel?: string;
};

export type LogFilterModel = {
  name: string;
  groups: string[];
};

export function getModelsForLogGroup(
  models: LogFilterModel[],
  groups: LogFilterGroup[],
  selectedGroup: string,
): LogFilterModel[] {
  if (!selectedGroup || selectedGroup === "auto") {
    return models;
  }

  const compositeGroup = groups.find(
    (group) => group.name === selectedGroup && group.composite === true,
  );
  if (compositeGroup) {
    if (!compositeGroup.publicModel) return [];
    return models.filter((model) => model.name === compositeGroup.publicModel);
  }

  return models.filter(
    (model) =>
      model.groups.includes(selectedGroup) || model.groups.includes("all"),
  );
}

export function mergeLogFilterModels(
  models: LogFilterModel[],
  groups: LogFilterGroup[],
): LogFilterModel[] {
  const modelGroups = new Map<string, Set<string>>();

  for (const model of models) {
    const groupsForModel = modelGroups.get(model.name) ?? new Set<string>();
    for (const group of model.groups) {
      groupsForModel.add(group);
    }
    modelGroups.set(model.name, groupsForModel);
  }

  for (const group of groups) {
    if (!group.composite || !group.publicModel) continue;
    const groupsForModel =
      modelGroups.get(group.publicModel) ?? new Set<string>();
    groupsForModel.add(group.name);
    modelGroups.set(group.publicModel, groupsForModel);
  }

  return [...modelGroups.entries()]
    .map(([name, modelGroupSet]) => ({
      name,
      groups: [...modelGroupSet].sort((a, b) => a.localeCompare(b)),
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
}
