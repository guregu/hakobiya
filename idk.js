function AnswerCtrl($scope, $routeParamaters, Hakobiya) {
	$scope.channel = $routeParamaters.id;
	$scope.users = 0;
	$scope.typing = false;
	$scope.answers = [];

	Hakobiya.bind($scope, $scope.channel, {
		"users": "$listeners",
		"typing": "%typing",
		"answers": "#answers"
	});
}