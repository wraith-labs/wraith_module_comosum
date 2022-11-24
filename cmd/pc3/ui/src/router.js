import { createRouter, createWebHashHistory } from 'vue-router';
import App from './App.vue';
import API from './api/api';

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

router.beforeEach(async (to) => {
    window.scrollTo(0, 0)

    // Make sure we can only see the dash when we're signed in.
    const api = new API()
    if (to.path !== '/login') {
        const authed = await api.checkauth()
        if (!authed) return { path: '/login' }
    } else {
        const authed = await api.checkauth()
        if (authed) return { path: '/' }
    }
});

export default router;
