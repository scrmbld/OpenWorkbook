const path = require("path")

module.exports = {
	entry: './src/js/index.js',
	output: {
		filename: 'bundle.js',
		path: path.resolve(__dirname, '../static'),
	},
	module: {
		rules: [
			{
				test: /\.css/i,
				use: ['style-loader', 'css-loader'],
			},
		],
	},
};
