const { FlatCompat } = require('@eslint/eslintrc');

const compat = new FlatCompat({ recommendedConfig: {} });

module.exports = [
    ...compat.config({
        extends: ['eslint:recommended', 'plugin:prettier/recommended'],
        ignorePatterns: ['coverage', 'var', 'vendor']
    })
];
