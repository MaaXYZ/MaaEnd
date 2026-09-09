import {rawJson, routes} from "./model.mjs";

export default routes.map((route) => ({
    RouteFileId: route.routeFileId,
    StartNode: route.startNode,
    GotoNode: route.gotoNode,
    EndNode: route.endNode,
    StartDescription: `AutoCollect 采集路线：传送到${route.name}（${route.campfireId}）`,
    GotoDescription: `采集${route.name}附近的 ${route.detailId}，共 ${route.count} 个`,
    TeleportParam: rawJson({sub: [route.enterNode]}),
    ActionParam: rawJson({path: route.path}),
}));
