import { createRouter, createWebHashHistory } from 'vue-router';
import App from './App.vue';

const routes = [
    {
        path: '/',
        name: 'app',
        component: App,
        children: [
            {
                path: '',
                component: () => import('./pages/Dashboard.vue')
            },
            {
                path: '/clients/manage',
                component: () => import('./pages/dashboard/ClientManager.vue')
            },
            {
                path: '/clients/console',
                component: () => import('./pages/dashboard/ClientConsole.vue')
            },
            {
                path: '/modules/manage',
                component: () => import('./pages/dashboard/ModuleManager.vue')
            },
            {
                path: '/modules/search',
                component: () => import('./pages/dashboard/ModuleIndex.vue')
            },
            {
                path: '/settings',
                component: () => import('./pages/dashboard/Settings.vue')
            },
            {
                path: '/about',
                component: () => import('./pages/dashboard/About.vue')
            }
        ]
    },
    {
        path: '/login',
        component: () => import('./pages/Login.vue')
    },
    {
        path: '/error',
        component: () => import('./pages/Error.vue')
    },
    {
        path: '/denied',
        component: () => import('./pages/Denied.vue')
    },
    {
        path: '/:pathMatch(.*)*',
        component: () => import('./pages/NotFound.vue')
    }
];

const router = createRouter({
    history: createWebHashHistory(),
    routes,
});

export default router;
