import { fetchDependencyReferences } from "app/backend";
import { fetchReferences, fetchXdefinition, fetchXreferences } from "app/backend/lsp";
import { addReferences, setReferences, store as referencesStore, ReferencesContext } from "app/references/store";

export function triggerReferences(context: ReferencesContext, refreshURL?: boolean): void {
	// TODO(john): make this caller's responsibility.
	if (refreshURL) {
		window.location.href = `${window.location.href.split("#")[0]}#L${context.loc.line}:${context.loc.char}$references`;
	}

	const repoRevSpec = {
		repoURI: context.loc.uri,
		rev: context.loc.rev,
		isBase: false,
		isDelta: false,
	}

	setReferences({ ...referencesStore.getValue(), context });
	fetchReferences(context.loc.char - 1, context.loc.path, context.loc.line - 1, repoRevSpec)
		.then((references) => {
			if (references) {
				addReferences(context.loc, references);
			}
		});
	fetchXdefinition(context.loc.char - 1, context.loc.path, context.loc.line - 1, repoRevSpec)
		.then(defInfo => {
			if (!defInfo) { return; }

			fetchDependencyReferences(repoRevSpec.repoURI, repoRevSpec.rev, context.loc.path, 40, 25).then((data) => {
				if (!data || !data.repoData.repos) { return; }
				const idToRepo = (id: number): any => {
					const i = data.repoData.repoIds.indexOf(id);
					if (i === -1) { throw new Error("repo id not found"); }
					return data.repoData.repos[i];
				};

				const retVal = data.dependencyReferenceData.references.map(ref => {
					const repo = idToRepo(ref.repoId);
					const commit = repo.lastIndexedRevOrLatest.commit;
					const workspace = commit ? { uri: repo.uri, rev: repo.lastIndexedRevOrLatest.commit.sha1 } : undefined;
					return {
						workspace,
						hints: ref.hints ? JSON.parse(ref.hints) : {},
					};
				}).filter(dep => dep.workspace); // possibly slice to MAX_DEPENDENT_REPOS (10)

				return retVal;
			}).then(dependents => {
				return dependents.map(dependent => {
					// const refs2Locations = (references: ReferenceInformation[]): vscode.Location[] => {
					// 	return references.map(r => this.currentWorkspaceClient.protocol2CodeConverter.asLocation(r.reference));
					// };
					// const params: WorkspaceReferencesParams = { query: defInfo.symbol, hints: dependent.hints, limit: 50 };
					return fetchXreferences(dependent.workspace, context.loc.path, defInfo.symbol, dependent.hints, 50).then((refs) => {
						if (refs) {
							addReferences(context.loc, refs)
						}
					});
				});
			});

		});
}
